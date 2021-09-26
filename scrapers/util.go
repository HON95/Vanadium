package scrapers

import (
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"dev.hon.one/vanadium/common"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type outputReaderStatus int

const (
	outputReaderOK outputReaderStatus = iota
	outputReaderDone
	outputReaderError
)

type outputReaderResult struct {
	Line   string
	Status outputReaderStatus
}

func checkDeviceFailure(device common.Device, message string, err error) bool {
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": device.Address,
		}).Tracef("Device error: %v", message)
		return false
	}
	return true
}

func showDeviceFailure(device common.Device, message string) {
	log.WithFields(log.Fields{
		"device": device.Address,
	}).Tracef("Device error: %v", message)
}

func openSSHClient(device common.Device) (*common.Credential, *ssh.Client, bool) {
	// Get credential
	credential, foundCredential := common.GlobalCredentials[device.CredentialID]
	if !foundCredential {
		log.WithFields(log.Fields{
			"device": device.Address,
		}).Warnf("Failed to find credential: %v", device.CredentialID)
		return nil, nil, false
	}

	// Setup SSH config
	authMethods := make([]ssh.AuthMethod, 0)
	if credential.Password != "" {
		authMethods = append(authMethods, ssh.Password(credential.Password))
	}
	if credential.PrivateKeyPath != "" {
		privkey, err := ioutil.ReadFile(credential.PrivateKeyPath)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"device": device.Address,
			}).Warnf("Failed to read SSH private key: %v", credential.PrivateKeyPath)
			return nil, nil, false
		}
		signer, err := ssh.ParsePrivateKey(privkey)
		if err != nil {
			log.WithError(err).WithFields(log.Fields{
				"device": device.Address,
			}).Warnf("Failed to parse SSH private key: %v", credential.PrivateKeyPath)
			return nil, nil, false
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}
	sshConfig := ssh.ClientConfig{
		User:            credential.Username,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Auth:            authMethods,
	}

	// Build full address
	port := uint(22)
	if device.Port > 0 {
		port = device.Port
	}
	fullAddress := fmt.Sprintf("%v:%v", device.Address, port)

	// Open connection
	sshClient, err := ssh.Dial("tcp", fullAddress, &sshConfig)
	if err != nil {
		log.WithError(err).WithFields(log.Fields{
			"device": device.Address,
		}).Tracef("Failed to connect to device: %v", fullAddress)
		return nil, nil, false
	}

	return &credential, sshClient, true
}

// Open SSH connection and run a single command.
// Appropriate if new connections are cheap for the device and shells are troublesome (output problems).
func runSSHCommand(device common.Device, command string) ([]string, bool) {
	// Setup
	_, sshClient, sshClientOpenSuccess := openSSHClient(device)
	if !sshClientOpenSuccess {
		return nil, false
	}
	defer sshClient.Close()
	session, err := sshClient.NewSession()
	if !checkDeviceFailure(device, "Failed to start session", err) {
		return nil, false
	}
	stdoutReader, err := session.StdoutPipe()
	if !checkDeviceFailure(device, "Failed to get STDOUT pipe", err) {
		return nil, false
	}
	stderrReader, err := session.StderrPipe()
	if !checkDeviceFailure(device, "Failed to get STDERR pipe", err) {
		return nil, false
	}

	// Run command
	err = session.Run(command)
	if !checkDeviceFailure(device, fmt.Sprintf("Failed to run SSH command: %v", command), err) {
		return nil, false
	}

	// Get output
	drainSSHStreamLines(device, "STDERR", stderrReader, false)
	lines, ok := collectSSHStreamLines(device, "STDOUT", stdoutReader)
	return lines, ok
}

// Reads the stream in the background and returns a channel for its lines.
func followSSHStreamLines(device common.Device, streamName string, reader io.Reader) <-chan outputReaderResult {
	outChannel := make(chan outputReaderResult, 256)

	go func() {
		bufferSize := 4096
		buffer := make([]byte, bufferSize)
		bufferUsage := 0
		for {
			// Read onto end of buffer
			tmpBuffer := buffer[bufferUsage:]
			tmpNumBytes, err := reader.Read(tmpBuffer)
			if err == io.EOF {
				outChannel <- outputReaderResult{Status: outputReaderDone}
				break
			} else if err != nil {
				checkDeviceFailure(device, fmt.Sprintf("Failed to read from %v stream", streamName), err)
				outChannel <- outputReaderResult{Status: outputReaderError}
			}
			bufferUsage += tmpNumBytes

			// Check for line
			for i := 0; i < bufferUsage; i++ {
				if buffer[i] == '\n' {
					line := string(buffer[:i])
					line = strings.Replace(line, "\r", "", -1)
					bufferUsage -= i + 1
					oldBufferRemains := buffer[i+1:]
					buffer = make([]byte, bufferSize)
					copy(buffer, oldBufferRemains)
					outChannel <- outputReaderResult{Line: line}
					break
				}
			}
		}
	}()

	return outChannel
}

// Reads the stream in the background and just prints them to log if anything appears.
func drainSSHStreamLines(device common.Device, streamName string, reader io.Reader, silent bool) {
	go func() {
		bufferSize := 4096
		buffer := make([]byte, bufferSize)
		bufferUsage := 0
		for {
			// Read onto end of buffer
			tmpBuffer := buffer[bufferUsage:]
			tmpNumBytes, err := reader.Read(tmpBuffer)
			if err == io.EOF {
				break
			} else if err != nil {
				checkDeviceFailure(device, fmt.Sprintf("Failed to read from %v stream", streamName), err)
			}
			bufferUsage += tmpNumBytes

			// Check for line
			for i := 0; i < bufferUsage; i++ {
				if buffer[i] == '\n' {
					line := string(buffer[:i])
					line = strings.Replace(line, "\r", "", -1)
					bufferUsage -= i + 1
					oldBufferRemains := buffer[i+1:]
					buffer = make([]byte, bufferSize)
					copy(buffer, oldBufferRemains)
					log.WithFields(log.Fields{
						"device": device.Address,
					}).Tracef("Received line on STDERR: %v", line)
					break
				}
			}
		}
	}()
}

// Blocks until reader is closed, then returns all lines from it.
func collectSSHStreamLines(device common.Device, streamName string, reader io.Reader) ([]string, bool) {
	mainBuffer := make([]byte, 4096)
	mainNumBytes := 0
	tmpBuffer := make([]byte, 4096)
	for {
		tmpNumBytes, err := reader.Read(tmpBuffer)
		if err == io.EOF {
			break
		} else if err != nil {
			checkDeviceFailure(device, fmt.Sprintf("Failed to read from %v stream", streamName), err)
			return nil, false
		}
		// Maybe expand main buffer (assume main buffer is at least as large as tmp buffer)
		if tmpNumBytes > len(mainBuffer)-mainNumBytes {
			oldMainBuffer := mainBuffer
			mainBuffer = make([]byte, 2*len(oldMainBuffer))
			copy(mainBuffer, oldMainBuffer)
		}
		// Copy into main buffer
		copy(mainBuffer[mainNumBytes:], tmpBuffer)
		mainNumBytes += tmpNumBytes
	}

	// Turn into text
	text := string(mainBuffer[:mainNumBytes])
	lines := strings.Split(text, "\n")

	return lines, true
}
