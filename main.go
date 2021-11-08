package main

import (
	"encoding/binary"
	"errors"
	"io"
	"os"

	"os/exec"

	"github.com/numtide/go-nix/wire"
	log "github.com/sirupsen/logrus"
)

// var WORKER_MAGIC_1 = []byte{0x63, 0x78, 0x69, 0x6e, 0x00, 0x00, 0x00, 0x00}
// var WORKER_MAGIC_2 = []byte{0x6f, 0x69, 0x78, 0x64, 0x00, 0x00, 0x00, 0x00}
// var PROTOCOL_VERSION = []byte{0x20, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
// var NULL32 = []byte{0x00, 0x00, 0x00, 0x00}
var CLIENT_VERSION uint64 = 0x10a

var WORKER_MAGIC_1 uint64 = 0x6e697863
var WORKER_MAGIC_2 uint64 = 0x6478696f
var PROTOCOL_VERSION uint64 = uint64((1<<8 | 32))

var STDERR_NEXT uint64 = 0x6f6c6d67
var STDERR_READ uint64 = 0x64617461  // data needed from source
var STDERR_WRITE uint64 = 0x64617416 // data for sink
var STDERR_LAST uint64 = 0x616c7473
var STDERR_ERROR uint64 = 0x63787470
var STDERR_START_ACTIVITY uint64 = 0x53545254
var STDERR_STOP_ACTIVITY uint64 = 0x53544f50
var STDERR_RESULT uint64 = 0x52534c54

var wopIsValidPath uint64 = 0x01

const storePath = "/home/tom/nix/st2"

func main() {
	var err error
	cmd := exec.Command("nix-daemon", "--stdio", "--store", storePath)

	pipeR, pipeW := io.Pipe()
	_, _ = pipeR, pipeW
	cmd.Stdout = io.MultiWriter(os.Stdout, pipeW)
	//cmd.Stdout = pipeW
	cmd.Stderr = os.Stderr
	fd, err := os.Create("pipe")
	check(err)

	inR, inW := io.Pipe()
	mw := io.MultiWriter(fd, inW)
	cmd.Stdin = inR
	in := mw

	//in, err := cmd.StdinPipe()
	check(err)
	out := pipeR
	//out, err := cmd.StdoutPipe()
	check(err)

	err = cmd.Start()
	check(err)
	err = writeUint64(in, WORKER_MAGIC_1)
	check(err)
	n, err := wire.ReadUint64(out)
	if err != nil || n != WORKER_MAGIC_2 {
		log.Fatalf("bad handshake: %s: %d", err.Error(), n)
	}
	n, err = wire.ReadUint64(out)
	if err != nil || n != PROTOCOL_VERSION {
		log.Fatalf("bad protocol: %s: %d", err.Error(), n)
	}
	err = writeUint64(in, CLIENT_VERSION)
	check(err)

	err = writeUint64(in, 0) // obsolete reserveSpace
	check(err)

	err = processError(out)
	check(err)

	err = writeUint64(in, wopIsValidPath)
	check(err)
	err = writeString(in, "/nix/store/zy9bcny22py17v0710c9rv2ib9jxa6pv-source.drv")
	check(err)

	err = processError(out)
	check(err)

	b, err := wire.ReadBool(out)
	check(err)
	log.Info(b)
}
func processError(out io.Reader) error {
	for {
		n, err := wire.ReadUint64(out)
		check(err)
		log.Errorf("%x", n)
		if n == STDERR_LAST {
			break
		}
		if n == STDERR_NEXT {
			str, err := wire.ReadString(out, 1024)
			if err != nil {
				return err
			}
			log.Error("next:", str)
			// n, err = wire.ReadUint64(out)
			// if err != nil {
			// 	return err
			// }
			// log.Error(n)
		}
		if n == STDERR_ERROR {
			str, err := wire.ReadString(out, 1024)
			if err != nil {
				return err
			}
			log.Error("error:", str)
			return errors.New(str)
		}
	}
	return nil
}

func writeUint64(in io.Writer, n uint64) error {
	return binary.Write(in, binary.LittleEndian, n)
}
func writeString(in io.Writer, str string) (err error) {
	l := uint64(len(str))
	err = binary.Write(in, binary.LittleEndian, l)
	if err != nil {
		return
	}
	err = binary.Write(in, binary.LittleEndian, []byte(str))
	if err != nil {
		return
	}
	mod := len(str) % 8
	if mod == 0 {
		return
	}
	for i := 0; i < 8-mod; i++ {
		err = binary.Write(in, binary.LittleEndian, []byte{0})
		if err != nil {
			return
		}
	}
	return
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
