# rptool
Extract files or mount ren'py archives (.rpa files)

This tool can extract files from ren'py archives, you can also use it to mount ren'py archives or directories that contain ren'py archives and look through them with your regular file browser.

Tested on MacOS, should work on Linux, extracting files should work in windows but not tested.

## Installation

`go install github.com/tw1nk/renpyarchivetool/cmd/rptool`

## Usage:

Extracting files from a specific archive to current directory:
`rptool extract path/to/archive.rpa`

Extracing files for all rpa files in a specific folder to to current directory:
`rptool extract path/to/game`

to switch output directory use the -o flag
example:
extract all files to ~/renpy-extract/examplegame
`rptool extract -o ~/renpy-extract/examplegame path/to/examplegame/images.rpa`


Mounting a specific rpa file:
`rptool mount path/to/archive.rpa path/to/mount`

Mounting all rpa files in in a specific folder:
``rptool mount path/to/game path/to/mount`

## But why?
I'm refuse to run python which is why I ported the functionality I needed from [github.com/Shizmob/rpatool](https://github.com/Shizmob/rpatool) to Go.

I added filetype detection to set correct filetype extensions of files which for one reason or another implies they are a different file type then they really are.