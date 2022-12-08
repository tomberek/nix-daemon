package main

import (
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"time"

	"github.com/nix-community/go-nix/pkg/wire"
	log "github.com/sirupsen/logrus"
)

// var WORKER_MAGIC_1 = []byte{0x63, 0x78, 0x69, 0x6e, 0x00, 0x00, 0x00, 0x00}
// var WORKER_MAGIC_2 = []byte{0x6f, 0x69, 0x78, 0x64, 0x00, 0x00, 0x00, 0x00}
// var PROTOCOL_VERSION = []byte{0x20, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
// var NULL32 = []byte{0x00, 0x00, 0x00, 0x00}
var CLIENT_VERSION uint64 = uint64((1<<8 | 34))

var WORKER_MAGIC_1 uint64 = 0x6e697863
var WORKER_MAGIC_2 uint64 = 0x6478696f
var PROTOCOL_VERSION uint64 = uint64((1<<8 | 34))

var STDERR_NEXT uint64 = 0x6f6c6d67
var STDERR_READ uint64 = 0x64617461  // data needed from source
var STDERR_WRITE uint64 = 0x64617416 // data for sink
var STDERR_LAST uint64 = 0x616c7473
var STDERR_ERROR uint64 = 0x63787470
var STDERR_START_ACTIVITY uint64 = 0x53545254
var STDERR_STOP_ACTIVITY uint64 = 0x53544f50
var STDERR_RESULT uint64 = 0x52534c54

var wopIsValidPath uint64 = 0x01
var wopAddBuildLog uint64 = 45

const storePath = "daemon"
const daemonSock = "/nix/var/nix/daemon-socket/socket"

func main() {
	var err error
	c, err := net.Dial("unix", daemonSock)
	if err != nil {
		log.Info("unable to connect to daemon socket")
		os.Exit(0)
	}
	// cmd := exec.Command("nix-daemon", "--stdio", "--store", storePath)

	//pipeR, pipeW := io.Pipe()
	//_, _ = pipeR, pipeW
	//cmd.Stdout = io.MultiWriter(os.Stdout, pipeW)
	////cmd.Stdout = pipeW
	//// cmd.Stderr = os.Stderr

	//fd, err := os.Create("pipe")
	//check(err)

	//inR, inW := io.Pipe()
	//mw := io.MultiWriter(fd, inW)
	//cmd.Stdin = inR
	//in := mw

	////in, err := cmd.StdinPipe()
	//check(err)
	//out := pipeR
	////out, err := cmd.StdoutPipe()
	//check(err)

	//err = cmd.Start()
	//if err != nil {
	//	log.Info("unable to talk to daemon")
	//	os.Exit(0)
	//}
	in := c
	out := c
	err = writeUint64(in, WORKER_MAGIC_1)
	check(err)
	n, err := wire.ReadUint64(out)
	if err != nil || n != WORKER_MAGIC_2 {
		log.Fatalf("bad handshake: %s: %d", err.Error(), n)
	}
	n, err = wire.ReadUint64(out)
	if err != nil || n > PROTOCOL_VERSION {
		log.Fatalf("bad protocol: %s: %d", err.Error(), n)
	}
	err = writeUint64(in, CLIENT_VERSION)
	check(err)

	err = writeUint64(in, 0) // obsolete CPU affinity
	check(err)
	err = writeUint64(in, 0) // obsolete reserveSpace
	check(err)

	i, err := wire.ReadBytesFull(out, 20)
	check(err)
	log.Info(string(i))
	err = processError(out)

	err = writeUint64(in, wopAddBuildLog)
	check(err)
	err = writeString(in, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-a.drv")
	check(err)

	// Exit if there is no error sent back
	go func() {
		time.Sleep(100 * time.Millisecond)
		os.Exit(0)
	}()

	// Otherwise, exit with error
	err = processError(out)
	check(err)
	os.Exit(1)

	// sending a fake log:
	// err = writeUint64(in, 1)
	// check(err)
	// err = writeString(in, "a")
	// check(err)
	// err = writeUint64(in, 0)
	// check(err)
}
func processError(out io.Reader) error {
	for {
		n, err := wire.ReadUint64(out)
		check(err)
		if err != nil {
			log.Errorf("%x", n)
		}
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
