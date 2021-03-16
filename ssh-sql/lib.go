package ssh_sql

import (
	"database/sql"
	"database/sql/driver"

	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/lib/pq"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

var db *sql.DB

type ConfSSH struct {
	Host string
	Port int
	User string
	Pass string
}

type ConfDB struct {
	Host   string
	User   string
	Pass   string
	DbName string
}

func GetConn(sshConf ConfSSH, dbConf ConfDB) (*ssh.Client, *sql.DB, error) {
	var agentClient agent.Agent
	if conn, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		defer conn.Close()
		agentClient = agent.NewClient(conn)
	} else {
		err = errors.New("SSH_SOCK is dial")
		return nil, nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User:            sshConf.User,
		Auth:            []ssh.AuthMethod{},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), //this line
	}

	if agentClient != nil {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeysCallback(agentClient.Signers))
	}

	if len(sshConf.Pass) != 0 {
		sshConfig.Auth = append(sshConfig.Auth, ssh.PasswordCallback(func() (string, error) {
			return sshConf.Pass, nil
		}))
	}

	if sshcon, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", sshConf.Host, sshConf.Port), sshConfig); err == nil {
		//defer sshcon.Close()

		sql.Register("postgres+ssh", &viaSSHDialer{sshcon})

		if db, err = sql.Open("postgres+ssh",
			fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable",
				dbConf.User, dbConf.Pass, dbConf.Host, dbConf.DbName),
		); err != nil {
			err = errors.New("Failed to connect to the db: " + err.Error())
			return nil, nil, err
		} else {
			return sshcon, db, nil
		}

	} else {
		err = errors.New("Failed to connect to the ssh: " + err.Error())
		return nil, nil, err
	}
}

type viaSSHDialer struct {
	client *ssh.Client
}

func (self *viaSSHDialer) Open(s string) (_ driver.Conn, err error) {
	return pq.DialOpen(self, s)
}

func (self *viaSSHDialer) Dial(network, address string) (net.Conn, error) {
	return self.client.Dial(network, address)
}

func (self *viaSSHDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return self.client.Dial(network, address)
}
