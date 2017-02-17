package acl

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	etcd "github.com/coreos/etcd/clientv3"
)

type Permission string

const PCreateUser Permission = "acl.user.add"
const PDeleteUser Permission = "acl.user.del"
const PAdmin Permission = "admin"
const PChangePassword Permission = "acl.user.passwd"

var PermissionDenied = errors.New("permission denied")

type ACLEngine struct {
	c *etcd.Client
}

func NewACLEngine(c *etcd.Client) *ACLEngine {
	return &ACLEngine{c: c}
}

func (e *ACLEngine) GetAllUsers() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := e.c.Get(ctx, "passwd/", etcd.WithPrefix())
	cancel()
	if err != nil {
		return nil, err
	}
	rv := []string{}
	re := regexp.MustCompile("^passwd/([a-z0-9_-]+)/hash$")
	for _, kv := range resp.Kvs {
		mr := re.FindStringSubmatch(string(kv.Key))
		if len(mr) == 0 {
			continue
		}
		rv = append(rv, mr[1])
	}
	return rv, nil
}

func (e *ACLEngine) CreateUser(username, password string) error {
	validUsername := regexp.MustCompile("^[a-z0-9_-]+$")
	if !validUsername.MatchString(username) {
		return errors.New("invalid username")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	hashkey := fmt.Sprintf("passwd/%s/hash", username)
	hashval, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	txr, err := e.c.Txn(ctx).
		If(etcd.Compare(etcd.Version(hashkey), "=", 0)).
		Then(etcd.OpPut(hashkey, string(hashval))).
		Commit()
	cancel()
	if err != nil {
		return err
	}
	if !txr.Succeeded {
		return errors.New("user exists")
	}
	return nil
}
func (e *ACLEngine) DeleteUser(username string) error {
	validUsername := regexp.MustCompile("^[a-z0-9_-]+$")
	if !validUsername.MatchString(username) {
		return errors.New("invalid username")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	hashkey := fmt.Sprintf("passwd/%s/hash", username)
	userkey := fmt.Sprintf("passwd/%s/", username)
	txr, err := e.c.Txn(ctx).
		If(etcd.Compare(etcd.Version(hashkey), ">", 0)).
		Then(etcd.OpDelete(userkey, etcd.WithPrefix())).
		Commit()
	cancel()
	if err != nil {
		return err
	}
	if !txr.Succeeded {
		return errors.New("user does not exist")
	}
	return nil
}
func (e *ACLEngine) SetPassword(username, password string) error {
	validUsername := regexp.MustCompile("^[a-z0-9_-]+$")
	if !validUsername.MatchString(username) {
		return errors.New("invalid username")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	hashkey := fmt.Sprintf("passwd/%s/hash", username)
	hashval, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	txr, err := e.c.Txn(ctx).
		If(etcd.Compare(etcd.Version(hashkey), ">", 0)).
		Then(etcd.OpPut(hashkey, string(hashval))).
		Commit()
	cancel()
	if err != nil {
		return err
	}
	if !txr.Succeeded {
		return errors.New("user does not exist")
	}
	return nil
}
func (e *ACLEngine) Test(username string, p Permission) error {
	if username == "admin" {
		fmt.Printf("[acl] testing for %q, username is %q -> OK\n", p, username)
		return nil
	}
	fmt.Printf("[acl] testing for %q, username is %q -> DENIED\n", p, username)
	return PermissionDenied
}
