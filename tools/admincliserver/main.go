package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"time"

	etcd "github.com/coreos/etcd/clientv3"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/ssh"
)

const VersionMajor = 4
const VersionMinor = 1
const VersionPatch = 2

var etcdClient *etcd.Client

var validUsername = regexp.MustCompile("^[a-z0-9_-]+$")

func checkBootstrapPassword() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := etcdClient.Get(ctx, "passwd/admin/hash")
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	if resp.Count != 0 {
		return
	}
	fmt.Printf("== WARNING, CREATING DEFAULT ADMIN ACCOUNT!! ==\n")
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	hsh, _ := bcrypt.GenerateFromPassword([]byte("sgs-default-admin-password"), bcrypt.DefaultCost)
	_, err = etcdClient.Put(ctx, "passwd/admin/hash", string(hsh))
	cancel()
	if err != nil {
		log.Fatal(err)
	}
}

func passwordAuth(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
	if !validUsername.MatchString(c.User()) {
		return nil, fmt.Errorf("invalid username %q", c.User())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := etcdClient.Get(ctx, fmt.Sprintf("passwd/%s/hash", c.User()))
	cancel()
	if err != nil {
		log.Fatal(err)
	}
	for _, ev := range resp.Kvs {
		err := bcrypt.CompareHashAndPassword(ev.Value, pass)
		if err != nil {
			return nil, fmt.Errorf("password rejected for %q", c.User())
		} else {
			fmt.Printf("[audit] password accepted for %q", c.User())
			return nil, nil
		}
	}
	return nil, fmt.Errorf("password rejected for %q", c.User())
}

func main() {
	if len(os.Args) == 2 && os.Args[1] == "-version" {
		fmt.Printf("%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
		os.Exit(0)
	}
	fmt.Printf("Booting console version %d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	var err error
	etcdEndpoint := os.Getenv("ETCD_ENDPOINT")
	if len(etcdEndpoint) == 0 {
		etcdEndpoint = "http://etcd:2379"
	}
	etcdClient, err = etcd.New(etcd.Config{
		Endpoints:   []string{etcdEndpoint},
		DialTimeout: 5 * time.Second})
	if err != nil {
		fmt.Printf("Could not connect to etcd: %v\n", err)
		os.Exit(1)
	}
	checkBootstrapPassword()
	InitRootModule(etcdClient)

	config := &ssh.ServerConfig{
		PasswordCallback: passwordAuth,
	}

	// You can generate a keypair with 'ssh-keygen -t rsa'
	privateBytes, err := ioutil.ReadFile("/etc/adminserver/id_rsa") //TODO
	if err != nil {
		log.Fatal("Failed to load private key (/etc/adminserver/id_rsa)")
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key")
	}

	config.AddHostKey(private)

	listener, err := net.Listen("tcp", "0.0.0.0:2222")
	if err != nil {
		log.Fatalf("Failed to listen on 2200 (%s)", err)
	}

	for {
		tcpConn, err := listener.Accept()
		if err != nil {
			log.Printf("Failed to accept incoming connection (%s)", err)
			continue
		}

		// Before use, a handshake must be performed on the incoming net.Conn.
		sshConn, chans, reqs, err := ssh.NewServerConn(tcpConn, config)
		if err != nil {
			log.Printf("Failed to handshake (%s)", err)
			continue
		}

		log.Printf("New admin console connection from %s (%s)", sshConn.RemoteAddr(), sshConn.ClientVersion())
		// Discard all global out-of-band Requests
		go ssh.DiscardRequests(reqs)
		// Accept all channels
		go handleChannels(chans, fmt.Sprintf("%s/%s", sshConn.User(), sshConn.RemoteAddr()))
	}
}

func handleChannels(chans <-chan ssh.NewChannel, id string) {
	// Service the incoming Channel channel in go routine
	for newChannel := range chans {
		go handleChannel(newChannel, id)
	}
}

func handleChannel(newChannel ssh.NewChannel, id string) {
	// Since we're handling a shell, we expect a
	// channel type of "session". The also describes
	// "x11", "direct-tcpip" and "forwarded-tcpip"
	// channel types.
	if t := newChannel.ChannelType(); t != "session" {
		newChannel.Reject(ssh.UnknownChannelType, fmt.Sprintf("unknown channel type: %s", t))
		return
	}

	// At this point, we have the opportunity to reject the client's
	// request for another logical connection
	connection, requests, err := newChannel.Accept()
	if err != nil {
		log.Printf("Could not accept channel (%s)", err)
		return
	}

	widthchan := make(chan int, 10)
	startS := func() {
		handleSession(connection, widthchan, id)
		connection.Close()
		log.Printf("Session closed")
	}

	// Sessions have out-of-band requests such as "shell", "pty-req" and "env"
	go func() {
		for req := range requests {
			switch req.Type {
			case "shell":
				// We only accept the default shell
				// (i.e. no command in the Payload)
				if len(req.Payload) == 0 {
					req.Reply(true, nil)
				}
				go startS()
			case "pty-req":
				termLen := req.Payload[3]
				w, h := parseDims(req.Payload[termLen+4:])
				_ = h
				widthchan <- int(w)

				// Responding true (OK) here will let the client
				// know we have a pty ready for input
				req.Reply(true, nil)
			case "window-change":
				w, h := parseDims(req.Payload)
				_ = h
				widthchan <- int(w)
			}
		}
	}()
}

// =======================

// parseDims extracts terminal dimensions (width x height) from the provided buffer.
func parseDims(b []byte) (uint32, uint32) {
	w := binary.BigEndian.Uint32(b)
	h := binary.BigEndian.Uint32(b[4:])
	return w, h
}
