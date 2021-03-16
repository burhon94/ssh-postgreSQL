package main

import (
	"database/sql"
	"database/sql/driver"

	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

func main() {

	var (
		sshHost = flag.String("sshHost", "127.0.0.1", "ssh host to connect")
		sshPort = flag.Int("sshPort", 1234, "ssh port to connect")
		sshUser = flag.String("sshUser", "root", "ssh user to connect")
		sshPass = flag.String("sshPass", "toor", "ssh pass to connect")
	)
	flag.Parse()

	dbUser := "postgres"  // DB username
	dbPass := "pass"      // DB Password
	dbHost := "localhost" // DB Hostname/IP
	dbName := "postgres"  // Database name

	var agentClient agent.Agent
	// Establish a connection to the local ssh-agent
	if conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		defer conn.Close()
		// Create a new instance of the ssh agent
		agentClient = agent.NewClient(conn)
	}

	// The client configuration with configuration option to use the ssh-agent
	sshConfig := &ssh.ClientConfig{
		User:            *sshUser,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //this line
	}

	// When the agentClient connection succeeded, add them as AuthMethod
	if agentClient != nil {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(agentClient.Signers))
	}
	// When there's a non empty password add the password AuthMethod
	if len(*sshPass) != 0 {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PasswordCallback(func() (string, error) {
			return *sshPass, nil
		}))
	}

	if sshcon, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", *sshHost, *sshPort), sshConfig); err == nil {
		defer sshcon.Close()

		sql.Register("postgres+ssh", &ViaSSHDialer{sshcon})

		if db, err := sql.Open("postgres+ssh", fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbUser, dbPass, dbHost, dbName)); err == nil {

			log.Printf("Successfully connected to the db\n")
			err := db.Ping()
			if err != nil {
				log.Fatalf("can't ping to db: %v", err)
				return
			}
			defer db.Close()

		} else {
			log.Printf("Failed to connect to the db: %s\n", err.Error())
		}
	} else {
		log.Printf("Failed to connect to the ssh: %s\n", err.Error())
	}
}

type ViaSSHDialer struct {
	client *ssh.Client
}

func (self *ViaSSHDialer) Open(s string) (_ driver.Conn, err error) {
	return pq.DialOpen(self, s)
}

func (self *ViaSSHDialer) Dial(network, address string) (net.Conn, error) {
	return self.client.Dial(network, address)
}

func (self *ViaSSHDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return self.client.Dial(network, address)
}
