package main

import (
	"strings"
	"testing"
)

const testXML = `<protocol name="test_proto">
  <interface name="wl_thing" version="3">
    <description summary="a thing"/>
    <request name="destroy" type="destructor"/>
    <request name="set_name" since="2">
      <arg name="name" type="string" allow-null="true"/>
      <arg name="other" type="object" interface="wl_thing" allow-null="true"/>
      <arg name="type" type="int"/>
    </request>
    <request name="make">
      <arg name="id" type="new_id" interface="wl_thing"/>
      <arg name="x" type="fixed"/>
    </request>
    <request name="bind">
      <arg name="name" type="uint"/>
      <arg name="id" type="new_id"/>
    </request>
    <event name="data">
      <arg name="fd" type="fd"/>
      <arg name="keys" type="array"/>
      <arg name="who" type="object" interface="foreign_iface"/>
    </event>
    <enum name="kind">
      <entry name="first_one" value="0x1" summary="first"/>
    </enum>
  </interface>
</protocol>`

func genTest(t *testing.T) string {
	t.Helper()
	p, err := parse([]byte(testXML))
	if err != nil {
		t.Fatal(err)
	}
	files, err := generate(p, knownInterfaces([]xProtocol{p}))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 || files["wl_thing_gen.go"] == nil || files["test_proto_tables_gen.go"] == nil {
		t.Fatalf("files = %d, want wl_thing_gen.go and test_proto_tables_gen.go", len(files))
	}
	return string(files["wl_thing_gen.go"]) + string(files["test_proto_tables_gen.go"])
}

// TestSignature pins the libwayland signature strings and types arrays:
// libwayland walks types by argument index, so one missing entry shifts
// every object argument after it.
func TestSignature(t *testing.T) {
	p, err := parse([]byte(testXML))
	if err != nil {
		t.Fatal(err)
	}
	known := knownInterfaces([]xProtocol{p})
	reqs := p.Interfaces[0].Requests
	tests := []struct {
		m         xMessage
		sig       string
		typeCount int
	}{
		{reqs[0], "", 0},
		{reqs[1], "2?s?oi", 3},
		{reqs[2], "nf", 2},
		{reqs[3], "usun", 4}, // generic new_id expands to s, u, n
		{p.Interfaces[0].Events[0], "hao", 3},
	}
	for _, tc := range tests {
		sig, types := signature(tc.m, known)
		if sig != tc.sig || len(types) != tc.typeCount {
			t.Errorf("%s: sig %q types %d, want %q %d", tc.m.Name, sig, len(types), tc.sig, tc.typeCount)
		}
	}
	if _, types := signature(reqs[1], known); types[1] != "&ThingInterface" {
		t.Errorf("object arg types entry = %s, want &ThingInterface", types[1])
	}
	// An interface outside the generated set must stay nil in the table.
	if _, types := signature(p.Interfaces[0].Events[0], known); types[2] != "nil" {
		t.Errorf("foreign interface types entry = %s, want nil", types[2])
	}
}

func TestGenerateShapes(t *testing.T) {
	src := genTest(t)
	for _, want := range []string{
		"type Thing struct{ Proxy }",
		// Destructor requests destroy the proxy.
		"px.marshal(0, nil, 0, marshalFlagDestroy)",
		// Nullable string, keyword arg renamed.
		"func (px Thing) SetName(name string, other Thing, type_ int32)",
		"nameC := cString(name, true)",
		// Typed new_id returns the new object with the parent's version.
		"func (px Thing) Make(x Fixed) Thing",
		"r := px.marshal(2, &ThingInterface, ver, 0, 0, uintptr(uint32(x)))",
		// Generic new_id takes the interface and version from the caller.
		"func (px Thing) Bind(name uint32, iface *Interface, version uint32) Proxy",
		// Event decoding: fd, borrowed array, object of an unknown interface.
		"Data func(fd int, keys []byte, who Proxy)",
		"h.Data(int(int32(uint32(a[0]))), arrayBytes(a[1]), Proxy{ptr: a[2]})",
		"ThingKindFirstOne = 0x1",
		`newMessage("set_name", "2?s?oi", []*Interface{nil, &ThingInterface, nil})`,
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated code lacks %q", want)
		}
	}
}

// TestDisplayEventsSkipped: libwayland owns wl_display's events, so no
// SetHandlers may be generated for it.
func TestDisplayEventsSkipped(t *testing.T) {
	xmlSrc := `<protocol name="p"><interface name="wl_display" version="1">
	  <request name="sync"><arg name="callback" type="new_id" interface="wl_display"/></request>
	  <event name="error"><arg name="code" type="uint"/></event>
	</interface></protocol>`
	p, err := parse([]byte(xmlSrc))
	if err != nil {
		t.Fatal(err)
	}
	files, err := generate(p, knownInterfaces([]xProtocol{p}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(files["wl_display_gen.go"]), "DisplayHandlers") {
		t.Error("wl_display got event handlers")
	}
}

func TestRequestNameCollision(t *testing.T) {
	xmlSrc := `<protocol name="p"><interface name="wl_x" version="1">
	  <request name="version"/></interface></protocol>`
	p, err := parse([]byte(xmlSrc))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := generate(p, knownInterfaces([]xProtocol{p})); err == nil {
		t.Error("a request named version must not shadow Proxy.Version")
	}
}

func TestNames(t *testing.T) {
	for in, want := range map[string]string{
		"wl_surface":        "Surface",
		"xdg_toplevel":      "XdgToplevel",
		"zwp_text_input_v3": "ZwpTextInputV3",
	} {
		if got := typeName(in); got != want {
			t.Errorf("typeName(%q) = %q, want %q", in, got, want)
		}
	}
	for in, want := range map[string]string{
		"interface":   "interface_",
		"serial":      "serial",
		"surface_x":   "surfaceX",
		"version":     "version_",
		"callback_id": "callbackId",
	} {
		if got := paramName(in); got != want {
			t.Errorf("paramName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseNoName(t *testing.T) {
	if _, err := parse([]byte(`<protocol></protocol>`)); err == nil {
		t.Error("want an error for a protocol with no name")
	}
}
