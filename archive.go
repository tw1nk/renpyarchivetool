package renpyarchivetool

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/nlpodyssey/gopickle/pickle"
	"github.com/nlpodyssey/gopickle/types"
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

func Load(fileName string) (*RenPyArchive, error) {
	rp := &RenPyArchive{}
	if err := rp.Load(fileName); err != nil {
		return nil, err
	}

	return rp, nil
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
				prefixBytes, ok := indexData.Get(2).([]byte)
				if !ok {
					return fmt.Errorf("prefix not not a string or []byte. %T", indexData.Get(2))
				}
				prefix = string(prefixBytes)
			}
			indexes[key].Prefix = []byte(prefix)
		}
	}

	rp.indexes = indexes

	return nil
}
