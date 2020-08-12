package main_test

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/zauberhaus/rest2dhcp/client"
	"github.com/zauberhaus/rest2dhcp/cmd"
	"github.com/zauberhaus/rest2dhcp/service"
	"github.com/zauberhaus/rest2dhcp/test"
	"github.com/zbiljic/go-filelock"
	"gopkg.in/yaml.v2"
)

func TestRunVersion(t *testing.T) {
	old := os.Stdout // keep backup of the real stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	service.Version = client.NewVersion(test.BuildDate, test.GitCommit, test.GitVersion, test.GitTreeState)
	cmd.VersionCmd.Run(nil, nil)

	outC := make(chan []byte)
	// copy the output in a separate goroutine so printing can't block indefinitely
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.Bytes()
	}()

	// back to normal state
	w.Close()
	os.Stdout = old // restoring the real stdout
	out := <-outC

	var info client.VersionInfo
	err := yaml.Unmarshal(out, &info)
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, info.ServiceVersion, "info.ServiceVersion is empty")
	assert.Equal(t, info.ServiceVersion, service.Version, "Invalid value")
}

func TestRunServer(t *testing.T) {
	pid := syscall.Getpid()
	done := make(chan bool)

	service.Version = client.NewVersion("", "", "", "")

	fl, err := filelock.New("/tmp/rest2dhcp-lock")
	if err != nil {
		panic(err)
	}

	err = fl.Lock()
	if err != nil {
		panic(err)
	}

	defer fl.Unlock()

	go func() {
		fmt.Println("Start server...")
		c := cmd.GetRootCmd()
		service.RunServer(c, nil)
		close(done)
	}()

	time.Sleep(5 * time.Second)

	resp, err := http.Get("http://localhost:8080/version")
	if err != nil {
		t.Fatal(err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Can't read version: %s (%v)", resp.Status, resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Can't read version: %s", err)
	}

	var info client.VersionInfo
	err = yaml.Unmarshal(data, &info)
	if err != nil {
		fmt.Println(string(data))
		t.Fatalf("Invalid version info: %s", err)
	}

	if info.ServiceVersion == nil || info.ServiceVersion.GoVersion == "" {
		fmt.Println(string(data))
		t.Fatal("Empty version info")
	}

	fmt.Printf("Send SIGINT to %v", pid)
	syscall.Kill(pid, syscall.SIGINT)

	<-done
	fmt.Println("Done.", pid)
}
