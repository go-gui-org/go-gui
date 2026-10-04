//go:build linux && !android

package keyring

import (
	"errors"
	"fmt"
	"time"

	"github.com/godbus/dbus/v5"

	"github.com/go-gui-org/go-gui/gui"
)

// Secret Service D-Bus API names
// (https://specifications.freedesktop.org/secret-service-spec/latest/).
const (
	ssDest       = "org.freedesktop.secrets"
	ssPath       = dbus.ObjectPath("/org/freedesktop/secrets")
	ssService    = "org.freedesktop.Secret.Service"
	ssCollection = "org.freedesktop.Secret.Collection"
	ssItem       = "org.freedesktop.Secret.Item"
	ssSession    = "org.freedesktop.Secret.Session"
	ssPrompt     = "org.freedesktop.Secret.Prompt"
	// noPrompt is the path a call returns when it needs no prompt.
	noPrompt = dbus.ObjectPath("/")
	// schema is the xdg:schema attribute, so items group under one
	// name in Seahorse and other keyring tools.
	schema = "org.go-gui.Secret"
	// promptTimeout bounds the wait for the user to answer an unlock
	// prompt. The call blocks the caller, so it must end.
	promptTimeout = 2 * time.Minute
)

// ssSecret is the Secret struct of the spec: (oayays).
type ssSecret struct {
	Session     dbus.ObjectPath
	Parameters  []byte
	Value       []byte
	ContentType string
}

// client is one Secret Service conversation: a private bus connection
// and an open session. Every call makes its own and closes it, so no
// connection or session state outlives the call.
type client struct {
	conn    *dbus.Conn
	svc     dbus.BusObject
	session dbus.ObjectPath
}

// open connects to the session bus and opens a "plain" session. The
// secret crosses only the user's own session bus, which other users
// cannot read; this is what most Secret Service clients use. A missing
// bus or daemon maps to ErrSecretsUnsupported.
func open() (*client, error) {
	conn, err := dbus.SessionBusPrivate()
	if err != nil {
		return nil, unsupported{reason: "no D-Bus session bus: " + err.Error()}
	}
	if err = conn.Auth(nil); err != nil {
		_ = conn.Close()
		return nil, unsupported{reason: "D-Bus auth: " + err.Error()}
	}
	if err = conn.Hello(); err != nil {
		_ = conn.Close()
		return nil, unsupported{reason: "D-Bus hello: " + err.Error()}
	}
	c := &client{conn: conn, svc: conn.Object(ssDest, ssPath)}
	var output dbus.Variant
	err = c.svc.Call(ssService+".OpenSession", 0, "plain", dbus.MakeVariant("")).
		Store(&output, &c.session)
	if err != nil {
		_ = conn.Close()
		return nil, mapErr("OpenSession", err)
	}
	return c, nil
}

func (c *client) close() {
	_ = c.conn.Object(ssDest, c.session).Call(ssSession+".Close", 0).Err
	_ = c.conn.Close()
}

// attrs are the lookup attributes of the item for service and key.
func attrs(service, key string) map[string]string {
	return map[string]string{"xdg:schema": schema, "application": service, "key": key}
}

// find returns the unlocked item for service and key, unlocking it if
// needed, or "" when there is none.
func (c *client) find(service, key string) (dbus.ObjectPath, error) {
	var unlocked, locked []dbus.ObjectPath
	err := c.svc.Call(ssService+".SearchItems", 0, attrs(service, key)).Store(&unlocked, &locked)
	if err != nil {
		return "", mapErr("SearchItems", err)
	}
	if len(unlocked) > 0 {
		return unlocked[0], nil
	}
	if len(locked) == 0 {
		return "", nil
	}
	if err = c.unlock(locked[:1]); err != nil {
		return "", err
	}
	return locked[0], nil
}

// unlock unlocks paths, showing the store's prompt if it asks for one.
func (c *client) unlock(paths []dbus.ObjectPath) error {
	var done []dbus.ObjectPath
	var prompt dbus.ObjectPath
	if err := c.svc.Call(ssService+".Unlock", 0, paths).Store(&done, &prompt); err != nil {
		return mapErr("Unlock", err)
	}
	return c.prompt(prompt)
}

// prompt runs a Secret Service prompt and waits for it to complete. A
// prompt of "/" means none is needed.
func (c *client) prompt(p dbus.ObjectPath) error {
	if p == noPrompt || p == "" {
		return nil
	}
	match := []dbus.MatchOption{
		dbus.WithMatchObjectPath(p),
		dbus.WithMatchInterface(ssPrompt),
		dbus.WithMatchMember("Completed"),
	}
	if err := c.conn.AddMatchSignal(match...); err != nil {
		return fmt.Errorf("secret service prompt: %w", err)
	}
	defer func() { _ = c.conn.RemoveMatchSignal(match...) }()
	ch := make(chan *dbus.Signal, 4)
	c.conn.Signal(ch)
	defer c.conn.RemoveSignal(ch)

	po := c.conn.Object(ssDest, p)
	if err := po.Call(ssPrompt+".Prompt", 0, "").Err; err != nil {
		return mapErr("Prompt", err)
	}
	timer := time.NewTimer(promptTimeout)
	defer timer.Stop()
	for {
		select {
		case sig, ok := <-ch:
			// godbus closes ch when the connection ends. A closed channel
			// reads nil at once, so without this check the loop would spin
			// until the timeout.
			if !ok {
				return errors.New("secret service: the connection closed during the unlock prompt")
			}
			if sig == nil || sig.Path != p || sig.Name != ssPrompt+".Completed" {
				continue
			}
			if len(sig.Body) > 0 {
				if dismissed, ok := sig.Body[0].(bool); ok && dismissed {
					return errors.New("secret service: the user dismissed the unlock prompt")
				}
			}
			return nil
		case <-timer.C:
			_ = po.Call(ssPrompt+".Dismiss", 0).Err
			return fmt.Errorf("secret service: no answer to the unlock prompt in %v", promptTimeout)
		}
	}
}

// Load returns the Secret Service item for service and key.
func Load(service, key string) ([]byte, error) {
	c, err := open()
	if err != nil {
		return nil, err
	}
	defer c.close()
	item, err := c.find(service, key)
	if err != nil {
		return nil, err
	}
	if item == "" {
		return nil, gui.ErrSecretNotFound
	}
	var s ssSecret
	if err = c.conn.Object(ssDest, item).Call(ssItem+".GetSecret", 0, c.session).Store(&s); err != nil {
		return nil, mapErr("GetSecret", err)
	}
	// An item another program wrote can be any size; gui would refuse it
	// anyway, but clear it here so the oversize copy does not linger.
	if len(s.Value) > maxItemBytes {
		clear(s.Value)
		return nil, fmt.Errorf("secret service: item is %d bytes, over the %d-byte limit",
			len(s.Value), maxItemBytes)
	}
	return s.Value, nil
}

// Save adds or replaces the item for service and key in the default
// collection.
func Save(service, key string, value []byte) error {
	c, err := open()
	if err != nil {
		return err
	}
	defer c.close()
	var coll dbus.ObjectPath
	if err = c.svc.Call(ssService+".ReadAlias", 0, "default").Store(&coll); err != nil {
		return mapErr("ReadAlias", err)
	}
	if coll == noPrompt || coll == "" {
		return unsupported{reason: "the Secret Service has no default collection"}
	}
	if err = c.unlock([]dbus.ObjectPath{coll}); err != nil {
		return err
	}
	props := map[string]dbus.Variant{
		ssItem + ".Label":      dbus.MakeVariant(service + "/" + key),
		ssItem + ".Attributes": dbus.MakeVariant(attrs(service, key)),
	}
	secret := ssSecret{
		Session:     c.session,
		Parameters:  []byte{},
		Value:       value,
		ContentType: "application/octet-stream",
	}
	var item, prompt dbus.ObjectPath
	// replace=true: an item with the same attributes is overwritten.
	err = c.conn.Object(ssDest, coll).Call(ssCollection+".CreateItem", 0, props, secret, true).
		Store(&item, &prompt)
	if err != nil {
		return mapErr("CreateItem", err)
	}
	return c.prompt(prompt)
}

// Delete removes the item for service and key. A missing item is not
// an error.
func Delete(service, key string) error {
	c, err := open()
	if err != nil {
		return err
	}
	defer c.close()
	item, err := c.find(service, key)
	if err != nil || item == "" {
		return err
	}
	var prompt dbus.ObjectPath
	if err = c.conn.Object(ssDest, item).Call(ssItem+".Delete", 0).Store(&prompt); err != nil {
		return mapErr("Delete", err)
	}
	return c.prompt(prompt)
}

// mapErr maps a D-Bus error to ErrSecretsUnsupported when no daemon owns
// the Secret Service name, and wraps it otherwise.
func mapErr(op string, err error) error {
	if de, ok := errors.AsType[dbus.Error](err); ok {
		switch de.Name {
		case "org.freedesktop.DBus.Error.ServiceUnknown",
			"org.freedesktop.DBus.Error.NameHasNoOwner",
			"org.freedesktop.DBus.Error.Spawn.ServiceNotFound":
			return unsupported{reason: "no Secret Service daemon (" + de.Name + ")"}
		}
	}
	return fmt.Errorf("secret service %s: %w", op, err)
}
