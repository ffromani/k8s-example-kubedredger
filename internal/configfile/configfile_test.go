/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package configfile

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr/testr"
	"github.com/google/go-cmp/cmp"
)

const (
	defaultConfName    = "workshop.conf"
	minimalConfContent = "[main]\nfoo=bar\n"
)

func TestStatusFromEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	st := mgr.Status(defaultConfName)
	exp := ConfigurationStatus{}

	if diff := cmp.Diff(st, exp); diff != "" {
		t.Errorf("unexpected status: %v", diff)
	}
}

func TestCreateFromScratch(t *testing.T) {
	lh := testr.New(t)
	ts := time.Now()
	time.Sleep(51 * time.Millisecond) // ensure update time diff

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	err := mgr.HandleSync(lh, ConfigRequest{
		Filename: defaultConfName,
		Content:  minimalConfContent,
		Create:   true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	confPath := filepath.Join(tmpDir, defaultConfName)
	st := mgr.Status(defaultConfName)
	verifyFileExistsWithContent(t, st, confPath, minimalConfContent, ts)
}

func TestCreateNotSet(t *testing.T) {
	lh := testr.New(t)

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	err := mgr.HandleSync(lh, ConfigRequest{
		Filename: defaultConfName,
		Content:  minimalConfContent,
		Create:   false,
	})
	if err == nil {
		t.Fatalf("create set, but file does not exist and this was allowed")
	}
}

func TestCreateDelete(t *testing.T) {
	lh := testr.New(t)
	ts := time.Now()
	time.Sleep(51 * time.Millisecond) // ensure update time diff

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	err := mgr.HandleSync(lh, ConfigRequest{
		Filename: defaultConfName,
		Content:  minimalConfContent,
		Create:   true,
	})
	if err != nil {
		t.Fatalf("unexpected sync error: %v", err)
	}

	confPath := filepath.Join(tmpDir, defaultConfName)
	st := mgr.Status(defaultConfName)
	verifyFileExistsWithContent(t, st, confPath, minimalConfContent, ts)

	err = mgr.Delete(defaultConfName)
	if err != nil {
		t.Fatalf("unexpected delete error: %v", err)
	}

	ok, err := FileExists(confPath)
	if err != nil {
		t.Fatalf("unexpected fileExists error: %v", err)
	}
	if ok {
		t.Fatalf("file exists after deletion")
	}
}

func TestCreateUpdate(t *testing.T) {
	lh := testr.New(t)
	ts := time.Now()
	time.Sleep(51 * time.Millisecond) // ensure update time diff

	tmpDir := t.TempDir()
	mgr := NewManager(tmpDir)
	err := mgr.HandleSync(lh, ConfigRequest{
		Filename: defaultConfName,
		Content:  minimalConfContent,
		Create:   true,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	confPath := filepath.Join(tmpDir, defaultConfName)
	st := mgr.Status(defaultConfName)
	verifyFileExistsWithContent(t, st, confPath, minimalConfContent, ts)

	time.Sleep(51 * time.Millisecond) // ensure update time diff
	content2 := `{\n"  foo": "bar"\n}`
	err = mgr.HandleSync(lh, ConfigRequest{
		Filename: defaultConfName,
		Content:  content2,
	})
	if err != nil {
		t.Fatalf("unexpected create error: %v", err)
	}

	st2 := mgr.Status(defaultConfName)
	verifyFileExistsWithContent(t, st2, confPath, content2, ts)
}

func verifyFileExistsWithContent(t *testing.T, st ConfigurationStatus, confPath, content string, ts time.Time) {
	t.Helper()
	bindata, err := os.ReadFile(confPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data := string(bindata)
	if data != content {
		t.Fatalf("unexpected content: got=%q wants=%q", data, content)
	}
	if st.FileUpdated.Before(ts) {
		t.Fatalf("unexpected update: got=%v ref=%v", st.FileUpdated, ts)
	}
	st.FileUpdated = ts // normalize
	expected := ConfigurationStatus{
		FileExists:  true,
		Content:     content,
		FileUpdated: ts,
	}
	if diff := cmp.Diff(st, expected); diff != "" {
		t.Fatalf("status mismatch: %v", diff)
	}
}
