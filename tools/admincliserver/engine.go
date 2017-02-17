package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"

	readline "github.com/chzyer/readline"
	"github.com/immesys/smartgridstore/admincli"
	shellwords "github.com/mattn/go-shellwords"
)

type sess struct {
	commandCancel func()
	path          []admincli.CLIModule
	rl            *readline.Instance
	parentCancel  func()
}

type interceptor struct {
	parentCancel   func()
	commandContext context.Context
	commandCancel  func()
	out            io.Writer
}

func (i *interceptor) Write(p []byte) (int, error) {
	if i.commandContext.Err() != nil {
		return 0, i.commandContext.Err()
	}
	oo := bytes.Replace(p, []byte("\n"), []byte("\r\n"), -1)
	n, err := i.out.Write(oo)
	if err != nil {
		fmt.Printf("output error: %v\n", err)
		i.parentCancel()
	}
	return n, err
}

func (s *sess) Do(line []rune, pos int) (newLine [][]rune, length int) {
	return nil, 0
}
func (s *sess) RePrompt() {
	els := []string{}
	for _, e := range s.path {
		if e.Name() != "" {
			els = append(els, e.Name())
		}
	}
	prompt := strings.Join(els, "/")
	prompt += "> "
	s.rl.SetPrompt(prompt)
}
func (s *sess) UpDir() {
	if len(s.path) > 1 {
		s.path = s.path[:len(s.path)-1]
		s.RePrompt()
	}
}
func (s *sess) CurrentModule() admincli.CLIModule {
	return s.path[len(s.path)-1]
}
func trWrite(w io.Writer, s string) {
	rs := strings.Replace(s, "\n", "\r\n", -1)
	w.Write([]byte(rs))
}
func (s *sess) List() string {
	var buffer bytes.Buffer
	childmax := 5
	for _, c := range s.CurrentModule().Children() {
		l := len(c.Name())
		if !c.Runnable() {
			l += 1
		}
		if l > childmax {
			childmax = l
		}
	}
	_ = childmax
	pfx := fmt.Sprintf("%%-%ds", childmax)
	for _, c := range s.CurrentModule().Children() {
		n := c.Name()
		if !c.Runnable() {
			n = n + "/"
		}
		if c.Hint() == "" {
			buffer.WriteString(fmt.Sprintf("%s\n", n))
		} else {
			buffer.WriteString(fmt.Sprintf(pfx+" - %s\n", n, c.Hint()))
		}
	}
	rv := buffer.String()
	return strings.TrimSuffix(rv, "\n")
}
func (s *sess) GetCmd(args []string) (admincli.CLIModule, string) {
	if strings.HasPrefix(strings.TrimSpace(args[0]), "#") {
		return nil, ""
	}
	switch args[0] {
	case "..":
		s.UpDir()
		return nil, ""
	case "cd":
		if len(args) != 2 {
			return nil, "usage: cd <module>"
		}
		if args[1] == ".." {
			s.UpDir()
			return nil, ""
		}
		for _, c := range s.CurrentModule().Children() {
			if args[1] == c.Name() {
				if c.Runnable() {
					return nil, fmt.Sprintf("'%s' is a command, not a category", args[1])
				}
				s.path = append(s.path, c)
				s.RePrompt()
				return nil, ""
			}
		}
		return nil, fmt.Sprintf("submodule '%s' not found", args[1])
	case "help", "man":
		if len(args) == 2 {
			for _, c := range s.CurrentModule().Children() {
				if args[1] == c.Name() {
					return nil, c.Name() + c.Usage()
				}
			}
			return nil, fmt.Sprintf("submodule '%s' not found", args[1])
		}
		return nil, fmt.Sprintf("%s%s\r\n%s", s.CurrentModule().Name(), s.CurrentModule().Usage(), s.List())
	case "ls", "l":
		return nil, s.List()
	case "exit", "quit":
		s.parentCancel()
		return nil, ""
	default:
		for _, c := range s.CurrentModule().Children() {
			if args[0] == c.Name() {
				if c.Runnable() {
					return c, ""
				} else {
					s.path = append(s.path, c)
					s.RePrompt()
					return nil, ""
				}
			}
		}
		return nil, fmt.Sprintf("submodule '%s' not found", args[0])
	}
}
func (s *sess) OnChange(line []rune, pos int, key rune) (newLine []rune, newPos int, ok bool) {
	if key == readline.CharInterrupt {
		fmt.Println("got inerrupt on listen")
		if s.commandCancel != nil {
			s.commandCancel()
		}
	}
	return nil, 0, false
}
func dummyF() error {
	return nil
}
func handleSession(link io.ReadWriteCloser, widthch chan int, user, ip string, root admincli.CLIModule) {
	s := &sess{}
	s.path = []admincli.CLIModule{root}
	var width uint64 = 80
	gotwidth := make(chan bool)
	go func() {
		hasclosed := false
		for w := range widthch {
			if w == -1 {
				w = 80
			}
			atomic.StoreUint64(&width, uint64(w))
			if !hasclosed {
				close(gotwidth)
				hasclosed = true
			}
		}
	}()
	select {
	case <-gotwidth:
	case <-time.After(1 * time.Second):
	}
	getWidth := func() int {
		i := atomic.LoadUint64(&width)
		return int(i)
	}
	logo := []string{
		"    _____   ______   ______  ",
		"   / ____| /  ____| /  ____| ",
		"  | (___   | |  __ | (____   ",
		"   \\___ \\  | | |_ | \\____ \\  ",
		"   ____) | | |__| |  ____) | ",
		"  |_____/  \\______| |_____/  ",
		"",
		" Smart Grid Store admin console",
		" (c) 2017 Michael Andersen, Sam Kumar",
		" (c) 2017 Regents of the University of California",
		"----------------------------------------------------",
		"",
	}
	for _, l := range logo {
		pad := (int(width) - len(l)) / 2
		pads := ""
		for i := 0; i < pad; i++ {
			pads += " "
		}
		fmt.Fprintf(link, "%s%s%s\r\n", pads, l, pads)
	}
	fIsTerminal := func() bool {
		return true
	}
	cfg := readline.Config{
		Prompt:              "> ",
		AutoComplete:        s,
		InterruptPrompt:     "^C",
		EOFPrompt:           "\r\nDisconnecting from SGS admin console",
		FuncGetWidth:        getWidth,
		Stdin:               link,
		Stdout:              link,
		Stderr:              link,
		Listener:            s,
		FuncIsTerminal:      fIsTerminal,
		FuncMakeRaw:         dummyF,
		FuncExitRaw:         dummyF,
		FuncOnWidthChanged:  nil,
		ForceUseInteractive: true,
	}
	inst, err := readline.NewEx(&cfg)
	s.rl = inst
	if err != nil {
		fmt.Printf("readline instantiation error: %v\n", err)
		return
	}
	prs := shellwords.NewParser()
	prs.ParseEnv = false
	prs.ParseBacktick = false
	parent, parentCancel := context.WithCancel(context.Background())
	s.parentCancel = parentCancel
	defer parentCancel()
	for {
		l, err := inst.Readline()
		fmt.Printf("[audit %s/%s] %s\n", user, ip, l)
		link.Write([]byte("\r"))
		//trWrite(link, "\n")
		if err != nil {
			if err == readline.ErrInterrupt {
				continue
			}
			fmt.Printf("whoops: %v\n", err)
			return
		}
		args, err := prs.Parse(l)
		if err != nil {
			trWrite(link, fmt.Sprintf("error: %v\n", err))
			continue
		}
		if len(args) == 0 {
			continue
		}
		cmd, msg := s.GetCmd(args)
		if parent.Err() != nil {
			fmt.Printf("parent context: %v\n", parent.Err())
			return
		} else {
			fmt.Printf("parent error is %v\n", parent.Err())
		}
		if msg != "" {
			if !strings.HasSuffix(msg, "\n") {
				msg += "\n"
			}
			trWrite(link, msg)
			continue
		}
		if cmd == nil {
			continue
		}
		ctx, cancel := context.WithCancel(parent)
		ctx = context.WithValue(ctx, admincli.ConsoleWidth, getWidth())
		//decode command
		icp := interceptor{
			parentCancel:   parentCancel,
			commandContext: ctx,
			commandCancel:  cancel,
			out:            link}
		s.commandCancel = cancel
		go func() {
			ok := cmd.Run(ctx, &icp, args[1:]...)
			if ctx.Err() != nil {
				return
			}
			if !ok {
				icp.Write([]byte(cmd.Name() + cmd.Usage()))
				if !strings.HasSuffix(cmd.Usage(), "\n") {
					icp.Write([]byte("\n"))
				}
			}
			cancel()
		}()
		<-ctx.Done()
		if parent.Err() != nil {
			fmt.Printf("parent context: %v", parent.Err())
			return
		}

	}
}
