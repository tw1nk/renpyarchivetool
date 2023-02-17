package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nlpodyssey/gopickle/pickle"
	"github.com/nlpodyssey/gopickle/types"

	"github.com/gabriel-vasile/mimetype"
)

type RenPyArchive struct {
	file     string
	handle   *os.File
	metadata string
	version  RPAVersion
	key      int64

	indexes map[string]*Index
}

type Index struct {
	Offset int64
	Length int64
	Prefix []byte
}

func (rp *RenPyArchive) Load(fileName string) error {
	var err error
	rp.file = fileName
	if rp.handle != nil {
		if err := rp.handle.Close(); err != nil {
			return fmt.Errorf("failed to close file. %v", err)
		}
	}

	rp.handle, err = os.Open(fileName)
	if err != nil {
		return fmt.Errorf("failed to open file: %s. %v", fileName, err)
	}

	if err := rp.getVersion(); err != nil {
		return fmt.Errorf("failed to get version. %v", err)
	}

	log.Println("archive version:", rp.version)

	if err := rp.extractIndexes(); err != nil {
		return fmt.Errorf("failed to extract indexes. %v", err)
	}

	return nil
}

const (
	RPA2Magic  = "RPA-2.0 "
	RPA3Magic  = "RPA-3.0 "
	RPA32Magic = "RPA-3.2 "
)

type RPAVersion int

const (
	RPAVersionUnknown RPAVersion = iota
	RPAVersion2
	RPAVersion3
	RPAVersion32
)

func (r RPAVersion) String() string {
	switch r {
	case RPAVersion2:
		return RPA2Magic
	case RPAVersion3:
		return RPA3Magic
	case RPAVersion32:
		return RPA32Magic
	}

	return "<unknown>"
}

func (rp *RenPyArchive) FileNames() []string {
	out := make([]string, 0)
	for fileName := range rp.indexes {
		out = append(out, fileName)
	}

	return out
}

func (rp *RenPyArchive) Read(filename string) ([]byte, error) {

	indexData, ok := rp.indexes[filename]
	if !ok {
		return nil, fmt.Errorf("file %s not found in archive", filename)
	}

	_, err := rp.handle.Seek(int64(indexData.Offset), 0)
	if err != nil {
		return nil, err
	}

	buf := make([]byte, indexData.Length)

	numRead, err := rp.handle.Read(buf)
	if err != nil {
		return nil, err
	}

	if int64(numRead) != indexData.Length {
		return nil, fmt.Errorf("didn't read full file. wanted to read: %d actually read: %d", indexData.Length, numRead)
	}

	return buf, nil
}

func (rp *RenPyArchive) getVersion() error {

	_, err := rp.handle.Seek(0, 0)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(rp.handle)
	scanner.Scan() // read 1 line into the scanner
	rp.metadata = scanner.Text()

	switch {
	case strings.HasPrefix(rp.metadata, RPA32Magic):
		rp.version = RPAVersion32
		return nil
	case strings.HasPrefix(rp.metadata, RPA3Magic):
		rp.version = RPAVersion3
		return nil
	case strings.HasPrefix(rp.metadata, RPA2Magic):
		rp.version = RPAVersion2
		return nil
	}

	return fmt.Errorf("the given file is not a valid Ren'Py archive, or an unsupported version")
}

func (rp *RenPyArchive) extractIndexes() error {
	vals := strings.Split(rp.metadata, " ")
	if len(vals) < 2 {
		return fmt.Errorf("invalid header format")
	}
	offset, err := strconv.ParseInt(vals[1], 16, 64)
	if err != nil {
		return nil
	}

	rp.key = 0

	if rp.version == RPAVersion3 {
		if len(vals) < 3 {
			return fmt.Errorf("invalid header format")
		}

		for i, subKeyString := range vals[2:] {
			subKey, err := strconv.ParseInt(subKeyString, 16, 64)
			if err != nil {
				return fmt.Errorf("failed to parse subkey %d. %v", i, err)
			}
			rp.key ^= int64(subKey)
		}
	} else if rp.version == RPAVersion32 {
		if len(vals) < 4 {
			return fmt.Errorf("invalid header format")
		}

		for i, subKeyString := range vals[3:] {
			subKey, err := strconv.ParseInt(subKeyString, 16, 64)
			if err != nil {
				return fmt.Errorf("failed to parse subkey %d. %v", i, err)
			}
			rp.key ^= int64(subKey)
		}
	}

	_, err = rp.handle.Seek(offset, 0)
	if err != nil {
		return fmt.Errorf("failed to seek to indexes. %v", err)
	}

	buf, err := io.ReadAll(rp.handle)
	if err != nil {
		return err
	}

	zlibReader, err := zlib.NewReader(bytes.NewReader(buf))
	if err != nil {
		return err
	}
	defer zlibReader.Close()

	unpickler := pickle.NewUnpickler(zlibReader)
	unpickled, err := unpickler.Load()
	if err != nil {
		return err
	}

	obfuscatedIndexs, ok := unpickled.(*types.Dict)
	if !ok {
		return fmt.Errorf("unpicked data not *types.Dict. was: %T", unpickled)
	}

	indexes := make(map[string]*Index)

	for _, keyInterface := range obfuscatedIndexs.Keys() {
		key, ok := keyInterface.(string)
		if !ok {
			return fmt.Errorf("key: %s wasn't a string. it's a %T", keyInterface, keyInterface)
		}

		interfaceValue, ok := obfuscatedIndexs.Get(key)
		if !ok {
			return fmt.Errorf("key: %s didn't exist in dict?", key)
		}
		value, ok := interfaceValue.(*types.List)
		if !ok {
			return fmt.Errorf("value for key: %s wasn't *types.List. was: %T", key, interfaceValue)
		}

		indexData, ok := value.Get(0).(*types.Tuple)
		if !ok {
			return fmt.Errorf("value.Get(0) for key: %s wasn't *types.Tuple. was: %T", key, value.Get(0))
		}

		indexOffset, ok := indexData.Get(0).(int)
		if !ok {
			return fmt.Errorf("indexOffset for key: %s wasn't int. was: %T", key, indexData.Get(0))
		}
		length, ok := indexData.Get(1).(int)
		if !ok {
			return fmt.Errorf("length for key: %s wasn't int. was: %T", key, indexData.Get(1))
		}

		indexes[key] = &Index{
			Offset: int64(indexOffset) ^ rp.key,
			Length: int64(length) ^ rp.key,
		}

		if indexData.Len() > 2 {
			prefix, ok := indexData.Get(2).(string)
			if !ok {
				return fmt.Errorf("not a string")
			}
			indexes[key].Prefix = []byte(prefix)
		}
	}

	rp.indexes = indexes

	return nil
}

var _ = pickle.Load

type argParams struct {
	extract      bool
	list         bool
	outputFolder string
}

func main() {
	params := argParams{
		list:         true,
		outputFolder: "./",
	}

	flag.BoolVar(&params.list, "list", true, "list files")
	flag.BoolVar(&params.extract, "extract", false, "extract files")
	flag.StringVar(&params.outputFolder, "out", ".", "where to extract files")
	flag.Parse()

	args := flag.Args()
	if len(args) < 1 {
		log.Println("filename must be provided")
	}
	filename := args[0]

	archive := RenPyArchive{}
	err := archive.Load(filename)
	if err != nil {
		log.Fatal(err)
	}

	if params.list {
		log.Println("Files in archive:")
		for _, fileName := range archive.FileNames() {
			log.Println("\t", fileName)
		}
	}

	if params.extract {
		var err error
		params.outputFolder, err = filepath.Abs(params.outputFolder)
		if err != nil {
			panic(err)
		}

		for _, filename := range archive.FileNames() {

			outputPath := filepath.Join(params.outputFolder, filename)
			dirName := filepath.Dir(outputPath)
			if err := os.MkdirAll(dirName, os.ModePerm); err != nil {
				panic(err)
			}

			fileData, err := archive.Read(filename)
			if err != nil {
				panic(err)
			}

			// handle things where the filetype doesn't match the extension
			fileType := mimetype.Detect(fileData)
			if !strings.HasSuffix(outputPath, fileType.Extension()) {
				outputPath += fileType.Extension()
			}

			log.Printf("Extracting %s filetype: %v", outputPath, fileType)

			err = os.WriteFile(outputPath, fileData, os.ModePerm)
			if err != nil {
				panic(err)
			}
		}
	}
}
