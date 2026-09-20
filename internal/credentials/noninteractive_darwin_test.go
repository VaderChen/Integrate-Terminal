//go:build darwin && cgo

package credentials

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"
)

type interactionTestDriver struct {
	allowed     bool
	readStatus  int32
	setStatuses []int32
	status      int32
	events      []string
	accessFlags []bool
	queryFlags  []bool
	onAccess    func(bool)
}

func (d *interactionTestDriver) interactionAllowed() (bool, int32) {
	d.events = append(d.events, "read-ui")
	return d.allowed, d.readStatus
}

func (d *interactionTestDriver) setInteractionAllowed(allowed bool) int32 {
	d.events = append(d.events, fmt.Sprintf("set-ui:%t", allowed))
	var status int32
	if len(d.setStatuses) != 0 {
		status, d.setStatuses = d.setStatuses[0], d.setStatuses[1:]
	}
	if status == 0 {
		d.allowed = allowed
	}
	return status
}

func (d *interactionTestDriver) access(operation string, noUI bool) int32 {
	d.events = append(d.events, operation)
	d.accessFlags = append(d.accessFlags, d.allowed)
	d.queryFlags = append(d.queryFlags, noUI)
	if d.onAccess != nil {
		d.onAccess(noUI)
	}
	return d.status
}

func (d *interactionTestDriver) get(_ string, noUI bool) ([]byte, int32) {
	return []byte("fake-secret"), d.access("get", noUI)
}
func (d *interactionTestDriver) put(_ string, _ []byte, noUI bool) int32 {
	return d.access("put", noUI)
}
func (d *interactionTestDriver) delete(_ string, noUI bool) int32 {
	return d.access("delete", noUI)
}

func TestNonInteractiveBackendRestoresOriginalFlagOnSuccessAndFailure(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		for _, status := range []int32{0, -25308} {
			for _, operation := range []string{"get", "put", "delete"} {
				t.Run(fmt.Sprintf("%s/original=%t/status=%d", operation, allowed, status), func(t *testing.T) {
					driver := &interactionTestDriver{allowed: allowed, status: status}
					backend := &keychainBackend{driver: driver, nonInteractive: true}
					var secret []byte
					var err error
					switch operation {
					case "get":
						secret, err = backend.Get("test/fake")
					case "put":
						err = backend.Put("test/fake", []byte("fake"))
					case "delete":
						err = backend.Delete("test/fake")
					}
					if (err == nil) != (status == 0) {
						t.Fatalf("status=%d error=%v", status, err)
					}
					if status != 0 && (secret != nil || !errors.Is(err, ErrLocked)) {
						t.Fatalf("failed access must return no secret and retain classification: %v", err)
					}
					if driver.allowed != allowed || !reflect.DeepEqual(driver.accessFlags, []bool{false}) || !reflect.DeepEqual(driver.queryFlags, []bool{true}) {
						t.Fatalf("wrong UI scope: current=%t native=%v query=%v", driver.allowed, driver.accessFlags, driver.queryFlags)
					}
					want := []string{"read-ui", "set-ui:false", operation, fmt.Sprintf("set-ui:%t", allowed)}
					if !reflect.DeepEqual(driver.events, want) {
						t.Fatalf("calls=%v want=%v", driver.events, want)
					}
				})
			}
		}
	}
}

func TestNonInteractiveBackendControlFailuresFailClosed(t *testing.T) {
	for _, test := range []struct {
		name        string
		driver      interactionTestDriver
		accessCalls int
	}{
		{"read-failure", interactionTestDriver{allowed: true, readStatus: -25291}, 0},
		{"disable-failure", interactionTestDriver{allowed: true, setStatuses: []int32{-25293}}, 0},
		{"restore-failure", interactionTestDriver{allowed: true, setStatuses: []int32{0, -25293}}, 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			driver := test.driver
			backend := &keychainBackend{driver: &driver, nonInteractive: true}
			secret, err := backend.Get("test/fake")
			if secret != nil || err == nil {
				t.Fatal("UI-control failure returned a successful credential")
			}
			if len(driver.accessFlags) != test.accessCalls {
				t.Fatalf("unexpected native calls: %v", driver.events)
			}
		})
	}
}

func TestInteractiveBackendPreservesExistingUIBehavior(t *testing.T) {
	for _, allowed := range []bool{false, true} {
		driver := &interactionTestDriver{allowed: allowed}
		backend := &keychainBackend{driver: driver}
		if _, err := backend.Get("test/fake"); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(driver.events, []string{"get"}) || driver.allowed != allowed || !reflect.DeepEqual(driver.queryFlags, []bool{false}) {
			t.Fatalf("interactive call changed UI state: %v", driver.events)
		}
	}
}

func TestInteractiveCallCannotOverlapSuppressedUIScope(t *testing.T) {
	entered := make(chan struct{})
	resume := make(chan struct{})
	interactiveEntered := make(chan struct{})
	driver := &interactionTestDriver{allowed: true, onAccess: func(noUI bool) {
		if noUI {
			close(entered)
			<-resume
		} else {
			close(interactiveEntered)
		}
	}}
	headless := &keychainBackend{driver: driver, nonInteractive: true}
	interactive := &keychainBackend{driver: driver}
	headlessDone := make(chan error, 1)
	interactiveDone := make(chan error, 1)
	go func() { _, err := headless.Get("test/fake"); headlessDone <- err }()
	<-entered
	go func() { _, err := interactive.Get("test/fake"); interactiveDone <- err }()
	select {
	case <-interactiveEntered:
		close(resume)
		<-headlessDone
		<-interactiveDone
		t.Fatal("interactive native call entered while UI was suppressed")
	case <-time.After(30 * time.Millisecond):
	}
	close(resume)
	if err := <-headlessDone; err != nil {
		t.Fatal(err)
	}
	if err := <-interactiveDone; err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(driver.accessFlags, []bool{false, true}) || !driver.allowed {
		t.Fatalf("UI was not restored before interactive native call: %v", driver.accessFlags)
	}
}

func TestNativeConstructorsChooseInteractionPolicy(t *testing.T) {
	if New().(*keychainBackend).nonInteractive || !NewNonInteractive().(*keychainBackend).nonInteractive {
		t.Fatal("backend constructors selected the wrong interaction policy")
	}
}
