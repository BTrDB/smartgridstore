package acl

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"regexp"
	"time"

	"golang.org/x/crypto/bcrypt"

	etcd "github.com/coreos/etcd/clientv3"
)

type ACLEngine struct {
	c      *etcd.Client
	prefix string
}

func NewACLEngine(prefix string, c *etcd.Client) *ACLEngine {
	return &ACLEngine{c: c, prefix: prefix}
}

type IdentityProvider string

var IDP_Invalid IdentityProvider = "invalid"
var IDP_Builtin IdentityProvider = "BuiltIn"
var IDP_LDAP IdentityProvider = "LDAP"

type Capability string

var KnownCapabilities = map[string]bool{
	"plotter":    true,
	"api":        true,
	"insert":     true,
	"read":       true,
	"delete":     true,
	"obliterate": true,
}

func (e *ACLEngine) set(key string, val string) error {
	_, err := e.c.Put(context.Background(), fmt.Sprintf("%s/%s", e.prefix, key), val)
	return err
}
func (e *ACLEngine) get(key string) ([]byte, error) {
	resp, err := e.c.Get(context.Background(), fmt.Sprintf("%s/%s", e.prefix, key))
	if err != nil {
		return nil, err
	}
	if resp.Count == 0 {
		return nil, nil
	}
	return resp.Kvs[0].Value, nil
}

func (e *ACLEngine) setstruct(key string, val interface{}) error {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(val)
	if err != nil {
		panic(err)
	}
	return e.set(key, string(buf.Bytes()))
}
func (e *ACLEngine) getstruct(key string, into interface{}) (bool, error) {
	arr, err := e.get(key)
	if err != nil {
		return false, err
	}
	if arr == nil {
		return false, nil
	}
	buf := bytes.NewBuffer(arr)
	dec := gob.NewDecoder(buf)
	err = dec.Decode(into)
	if err != nil {
		return false, err
	}
	return true, nil
}
func (e *ACLEngine) GetIDP() (IdentityProvider, error) {
	idp, err := e.get("auth/idpmode")
	if err != nil {
		return IDP_Invalid, err
	}
	if idp == nil {
		e.SetIDP(IDP_Builtin)
		return IDP_Builtin, nil
	}
	return IdentityProvider(idp), nil
}
func (e *ACLEngine) SetIDP(p IdentityProvider) error {
	return e.set("auth/idpmode", string(p))
}

type Group struct {
	Name         string
	Prefixes     []string
	Capabilities []string
}

func (e *ACLEngine) GetGroups() ([]*Group, error) {
	resp, err := e.c.Get(context.Background(), fmt.Sprintf("%s/auth/groups", e.prefix), etcd.WithPrefix())
	if err != nil {
		return nil, err
	}
	rv := []*Group{}
	for _, rec := range resp.Kvs {
		g := Group{}
		buf := bytes.NewBuffer(rec.Value)
		dec := gob.NewDecoder(buf)
		err := dec.Decode(&g)
		if err != nil {
			return nil, err
		}
		rv = append(rv, &g)
	}
	return rv, nil
}
func (e *ACLEngine) GetGroup(name string) (*Group, error) {
	g := &Group{}
	found, err := e.getstruct(fmt.Sprintf("auth/groups/%s", name), g)
	if err != nil {
		return nil, err
	}
	if !found {
		if name == "public" {
			//The public group always exists
			return &Group{
				Name:         "public",
				Capabilities: []string{"plotter"},
				Prefixes:     []string{""},
			}, nil
		}
		return nil, nil
	}
	return g, nil
}
func (e *ACLEngine) AddGroup(name string) error {
	g := &Group{
		Name: name,
	}
	pat := regexp.MustCompile("^[a-zA-Z0-9]+$")
	if !pat.MatchString(name) {
		return fmt.Errorf("invalid group name %q", name)
	}
	return e.setstruct(fmt.Sprintf("auth/groups/%s", name), g)
}

func (e *ACLEngine) DeleteGroup(name string) error {
	g, err := e.GetGroup(name)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group not found\n")
	}
	_, err = e.c.Delete(context.Background(), fmt.Sprintf("auth/groups/%s", name))
	return err
}

func (e *ACLEngine) AddPrefixToGroup(group string, prefix string) error {
	g, err := e.GetGroup(group)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group not found")
	}
	for _, pfx := range g.Prefixes {
		if pfx == prefix {
			return nil
		}
	}
	g.Prefixes = append(g.Prefixes, prefix)
	return e.setstruct(fmt.Sprintf("auth/groups/%s", group), g)
}
func (e *ACLEngine) RemovePrefixFromGroup(group string, prefix string) error {
	g, err := e.GetGroup(group)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group not found")
	}
	newprefixes := []string{}
	for _, pfx := range g.Prefixes {
		if pfx == prefix {
			continue
		}
		newprefixes = append(newprefixes, pfx)
	}
	return e.setstruct(fmt.Sprintf("auth/groups/%s", group), g)
}

func (e *ACLEngine) AddCapabilityToGroup(group string, capability string) error {
	if !KnownCapabilities[capability] {
		return fmt.Errorf("unknown capability %q", capability)
	}
	g, err := e.GetGroup(group)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group not found")
	}
	for _, cap := range g.Capabilities {
		if cap == capability {
			return nil
		}
	}
	g.Capabilities = append(g.Capabilities, capability)
	return e.setstruct(fmt.Sprintf("auth/groups/%s", group), g)
}
func (e *ACLEngine) RemoveCapabilityFromGroup(group string, capability string) error {
	g, err := e.GetGroup(group)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group not found")
	}
	newcaps := []string{}
	for _, cap := range g.Capabilities {
		if cap == capability {
			continue
		}
		newcaps = append(newcaps, cap)
	}
	return e.setstruct(fmt.Sprintf("auth/groups/%s", group), g)
}

func (e *ACLEngine) AddUserToGroup(user string, group string) error {
	g, err := e.GetGroup(group)
	if err != nil {
		return err
	}
	if g == nil {
		return fmt.Errorf("group not found")
	}
	idp, err := e.GetIDP()
	if err != nil {
		return err
	}
	if idp != IDP_Builtin {
		return fmt.Errorf("cannot add a user to a group when not using the built-in identity provider")
	}
	bu := &BuiltinUser{}
	found, err := e.getstruct(fmt.Sprintf("auth/users/%s", user), bu)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("user does not exist\n")
	}
	for _, g := range bu.Groups {
		if g == group {
			return nil
		}
	}
	bu.Groups = append(bu.Groups, group)
	return e.setstruct(fmt.Sprintf("auth/users/%s", user), bu)
}
func (e *ACLEngine) RemoveUserFromGroup(user string, group string) error {
	idp, err := e.GetIDP()
	if err != nil {
		return err
	}
	if idp != IDP_Builtin {
		return fmt.Errorf("cannot remove a user from a group when not using the built-in identity provider")
	}
	bu := &BuiltinUser{}
	found, err := e.getstruct(fmt.Sprintf("auth/users/%s", user), bu)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("user does not exist\n")
	}
	newgroups := []string{}
	for _, g := range bu.Groups {
		if g == group {
			continue
		}
		newgroups = append(newgroups, g)
	}
	bu.Groups = newgroups
	return e.setstruct(fmt.Sprintf("auth/users/%s", user), bu)
}

type BuiltinUser struct {
	Groups   []string
	Password string
}

type User struct {
	Groups   []string
	Password string

	//Calculated at load time
	Prefixes     []string
	Capabilities []string
}

func (e *ACLEngine) WatchForAuthChanges(ctx context.Context) (chan struct{}, error) {
	rv := make(chan struct{}, 10)
	go func() {
		wc := e.c.Watch(ctx, fmt.Sprintf("%s/auth/", e.prefix), etcd.WithPrefix())
		for _ = range wc {
			rv <- struct{}{}
		}
		panic("watch ended")
	}()
	return rv, nil
}

//Returns false, nil, nil if password is incorrect or user does not exist
func (e *ACLEngine) AuthenticateUser(name string, password string) (bool, *User, error) {
	idp, err := e.GetIDP()
	if err != nil {
		return false, nil, err
	}
	if idp == IDP_Builtin {
		u, err := e.GetBuiltinUser(name)
		if err != nil {
			return false, nil, err
		}
		if u == nil {
			return false, nil, nil
		}
		err = bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
		if err != nil {
			return false, nil, nil
		}
		return true, u, nil
	}
	return false, nil, fmt.Errorf("unsupported identity provider")
}
func (e *ACLEngine) GetBuiltinUser(name string) (*User, error) {
	bu := &BuiltinUser{}
	found, err := e.getstruct(fmt.Sprintf("auth/users/%s", name), bu)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	rv := &User{
		Password: bu.Password,
		Groups:   bu.Groups,
	}
	pfxs := make(map[string]bool)
	caps := make(map[string]bool)
	for _, gs := range bu.Groups {
		g, err := e.GetGroup(gs)
		if err != nil {
			return nil, err
		}
		for _, p := range g.Prefixes {
			pfxs[p] = true
		}
		for _, p := range g.Capabilities {
			caps[p] = true
		}
	}
	for cap, _ := range caps {
		rv.Capabilities = append(rv.Capabilities, cap)
	}
	for pfx, _ := range pfxs {
		rv.Prefixes = append(rv.Prefixes, pfx)
	}
	return rv, nil
}
func (e *ACLEngine) ConstructUser(groups []string) (*User, error) {
	rv := &User{
		Groups: groups,
	}
	pfxs := make(map[string]bool)
	caps := make(map[string]bool)
	for _, gs := range groups {
		g, err := e.GetGroup(gs)
		if err != nil {
			return nil, err
		}
		for _, p := range g.Prefixes {
			pfxs[p] = true
		}
		for _, p := range g.Capabilities {
			caps[p] = true
		}
	}
	for cap, _ := range caps {
		rv.Capabilities = append(rv.Capabilities, cap)
	}
	for pfx, _ := range pfxs {
		rv.Prefixes = append(rv.Prefixes, pfx)
	}
	return rv, nil
}
func (e *ACLEngine) GetAllUsers() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	resp, err := e.c.Get(ctx, fmt.Sprintf("%s/auth/users/", e.prefix), etcd.WithPrefix())
	cancel()
	if err != nil {
		return nil, err
	}
	rv := []string{}
	re := regexp.MustCompile("auth/users/(.*)$")
	for _, kv := range resp.Kvs {
		mr := re.FindStringSubmatch(string(kv.Key))
		if len(mr) == 0 {
			continue
		}
		rv = append(rv, mr[1])
	}
	return rv, nil
}

func (e *ACLEngine) CreateDefaultAdminUser(password string) error {
	hashval, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	bu := &BuiltinUser{}
	found, err := e.getstruct("auth/users/admin", bu)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("user exists\n")
	}
	bu.Password = string(hashval)
	return e.setstruct("auth/users/admin", bu)
}
func (e *ACLEngine) CreateUser(username, password string) error {
	idp, err := e.GetIDP()
	if err != nil {
		return err
	}
	if idp != IDP_Builtin {
		return fmt.Errorf("you cannot create users if not using the builtin identity provider\n")
	}
	validUsername := regexp.MustCompile("^[a-z0-9_-]+$")
	if !validUsername.MatchString(username) {
		return errors.New("invalid username")
	}
	hashval, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	bu := &BuiltinUser{}
	found, err := e.getstruct(fmt.Sprintf("auth/users/%s", username), bu)
	if err != nil {
		return err
	}
	if found {
		return fmt.Errorf("user exists\n")
	}
	bu.Password = string(hashval)

	return e.setstruct(fmt.Sprintf("auth/users/%s", username), bu)
}
func (e *ACLEngine) DeleteUser(username string) error {
	idp, err := e.GetIDP()
	if err != nil {
		return err
	}
	if idp != IDP_Builtin {
		return fmt.Errorf("you cannot delete users if not using the builtin identity provider\n")
	}
	validUsername := regexp.MustCompile("^[a-z0-9_-]+$")
	if !validUsername.MatchString(username) {
		return errors.New("invalid username")
	}
	bu := &BuiltinUser{}
	found, err := e.getstruct(fmt.Sprintf("auth/users/%s", username), bu)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("user does not exist\n")
	}
	_, err = e.c.Delete(context.Background(), fmt.Sprintf("%s/auth/users/%s", e.prefix, username))
	return err
}
func (e *ACLEngine) SetPassword(username, password string) error {
	if username != "admin" {
		idp, err := e.GetIDP()
		if err != nil {
			return err
		}
		if idp != IDP_Builtin {
			return fmt.Errorf("you cannot set user passwords if not using the builtin identity provider\n")
		}
	}
	validUsername := regexp.MustCompile("^[a-z0-9_-]+$")
	if !validUsername.MatchString(username) {
		return errors.New("invalid username")
	}
	hashval, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	bu := &BuiltinUser{}
	found, err := e.getstruct(fmt.Sprintf("auth/users/%s", username), bu)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("user does not exist\n")
	}
	bu.Password = string(hashval)

	return e.setstruct(fmt.Sprintf("auth/users/%s", username), bu)
}
