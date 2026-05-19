package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
)

const SubcommandName = "worker"

type Client struct {
	selfPath string
	sudoPath string
}

func NewClient() (*Client, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate self binary: %w", err)
	}
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return nil, fmt.Errorf("sudo not found in PATH: %w", err)
	}
	return &Client{selfPath: self, sudoPath: sudo}, nil
}

func (c *Client) ReadUUID(devPath string) (string, error) {
	resp, err := c.run(&Request{Op: OpReadUUID, Dev: devPath}, nil)
	if err != nil {
		return "", err
	}
	if !resp.OK {
		return "", errors.New(resp.Message)
	}
	return resp.Message, nil
}

func (c *Client) UnlockAndMount(req *Request, passphrase []byte) error {
	if req.Op == "" {
		req.Op = OpUnlockAndMount
	}
	resp, err := c.run(req, passphrase)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Message)
	}
	return nil
}

func (c *Client) UnmountAndClose(req *Request) error {
	if req.Op == "" {
		req.Op = OpUnmountAndClose
	}
	resp, err := c.run(req, nil)
	if err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Message)
	}
	return nil
}

func (c *Client) run(req *Request, passphrase []byte) (*Response, error) {
	cmd := exec.Command(c.sudoPath, c.selfPath, SubcommandName)
	cmd.Stderr = os.Stderr

	var input bytes.Buffer
	jb, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	input.Write(jb)
	input.WriteByte('\n')
	if passphrase != nil {
		input.Write(passphrase)
		input.WriteByte('\n')
	}
	cmd.Stdin = &input

	var out bytes.Buffer
	cmd.Stdout = &out

	runErr := cmd.Run()
	if errors.Is(runErr, exec.ErrNotFound) {
		return nil, runErr
	}
	if runErr != nil {
		var ee *exec.ExitError
		if !errors.As(runErr, &ee) {
			return nil, fmt.Errorf("worker process: %w", runErr)
		}
	}

	if out.Len() == 0 {
		if runErr != nil {
			return nil, fmt.Errorf("worker exited without response: %w", runErr)
		}
		return nil, errors.New("worker produced no response")
	}

	var resp Response
	if err := json.Unmarshal(bytes.TrimSpace(out.Bytes()), &resp); err != nil {
		return nil, fmt.Errorf("parse worker response: %w", err)
	}
	return &resp, nil
}

func RunInteractiveSudo() error {
	sudo, err := exec.LookPath("sudo")
	if err != nil {
		return err
	}
	cmd := exec.Command(sudo, "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
