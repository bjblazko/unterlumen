package installation

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// isolate points the user's configuration folder at a temporary one.
func isolate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
}

func TestLoadWithoutAFileIsAFirstStart(t *testing.T) {
	isolate(t)
	if _, found, err := Load(); found || err != nil {
		t.Errorf("found=%v err=%v, want a first start", found, err)
	}
}

func TestSaveThenLoad(t *testing.T) {
	isolate(t)
	want := Config{PhotosDir: "/p", LibDir: "/l", ChannelsDir: "/p/.unterlumen-shared", Port: 8090}
	if err := Save(want); err != nil {
		t.Fatal(err)
	}
	got, found, err := Load()
	if !found || err != nil || got != want {
		t.Errorf("Load = %+v, %v, %v; want %+v", got, found, err, want)
	}
}

func TestFromLauncher(t *testing.T) {
	cases := []struct {
		name, script string
		want         Config
	}{
		{"macOS", "#!/bin/bash\nexport PATH=\"/opt/homebrew/bin:$PATH\"\nDIR=\"$(cd \"$(dirname \"$0\")\" && pwd)\"\nexec \"$DIR/unterlumen\" -desktop -port 8090 -lib-dir '/Users/me/Library/Application Support/Unterlumen' -channels-dir /Volumes/nas/Bilder/.unterlumen-shared '/'\n",
			Config{Port: 8090, LibDir: "/Users/me/Library/Application Support/Unterlumen", ChannelsDir: "/Volumes/nas/Bilder/.unterlumen-shared", PhotosDir: "/"}},
		{"Linux with a quote in the path", "#!/bin/bash\nexec \"$DIR/unterlumen\" -desktop -port 8091 -lib-dir '/home/me/.local/share/unterlumen' '/home/me/Tim'\\''s photos'\n",
			Config{Port: 8091, LibDir: "/home/me/.local/share/unterlumen", PhotosDir: "/home/me/Tim's photos"}},
		{"Windows", "@echo off\r\n\"C:\\Users\\me\\AppData\\Local\\Unterlumen\\unterlumen.exe\" -desktop -port 8090 -lib-dir \"C:\\Users\\me\\AppData\\Roaming\\Unterlumen\" \"C:\\Users\\me\\Pictures\"\r\n",
			Config{Port: 8090, LibDir: `C:\Users\me\AppData\Roaming\Unterlumen`, PhotosDir: `C:\Users\me\Pictures`}},
		{"new launcher without flags", "#!/bin/bash\nexec \"$DIR/unterlumen\" -desktop\n", Config{}},
	}
	for _, c := range cases {
		got, ok := FromLauncher(c.script)
		if !ok || got != c.want {
			t.Errorf("%s: %+v, %v; want %+v", c.name, got, ok, c.want)
		}
	}
	if _, ok := FromLauncher("#!/bin/bash\necho hello\n"); ok {
		t.Error("a script without -desktop read as a launcher")
	}
}

func TestFindShared(t *testing.T) {
	photos := t.TempDir()
	if got := FindShared(photos); got != "" {
		t.Errorf("FindShared without the folder = %q", got)
	}
	os.Mkdir(filepath.Join(photos, SharedDirName), 0o755) //nolint:errcheck
	if got := FindShared(photos); got != filepath.Join(photos, SharedDirName) {
		t.Errorf("FindShared = %q", got)
	}
	if got := FindShared(""); got != "" {
		t.Errorf("FindShared(\"\") = %q", got)
	}
}

func TestShareCopiesDestinationsAndAlbums(t *testing.T) {
	photos, lib := t.TempDir(), t.TempDir()
	os.WriteFile(filepath.Join(lib, "channels.json"), []byte(`{"local":true}`), 0o644) //nolint:errcheck
	os.MkdirAll(filepath.Join(lib, "albums", "site"), 0o755)                           //nolint:errcheck
	os.WriteFile(filepath.Join(lib, "albums", "site", "a.json"), []byte(`{}`), 0o644)  //nolint:errcheck

	dir, err := Share(photos, lib)
	if err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "channels.json")); string(data) != `{"local":true}` {
		t.Errorf("channels.json = %q", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "albums", "site", "a.json")); err != nil {
		t.Errorf("album register not copied: %v", err)
	}
}

func TestShareNeverOverwritesWhatIsShared(t *testing.T) {
	photos, lib := t.TempDir(), t.TempDir()
	shared := filepath.Join(photos, SharedDirName)
	os.MkdirAll(shared, 0o755)                                                             //nolint:errcheck
	os.WriteFile(filepath.Join(shared, "channels.json"), []byte(`{"shared":true}`), 0o644) //nolint:errcheck
	os.WriteFile(filepath.Join(lib, "channels.json"), []byte(`{"local":true}`), 0o644)     //nolint:errcheck

	if _, err := Share(photos, lib); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(shared, "channels.json")); string(data) != `{"shared":true}` {
		t.Errorf("shared channels.json overwritten: %q", data)
	}
}

func TestDecide(t *testing.T) {
	photos, lib := t.TempDir(), t.TempDir()
	shared := filepath.Join(photos, SharedDirName)

	got, err := Decide(Config{Port: 8090}, Choice{PhotosDir: photos}, lib, lib)
	if err != nil || got != (Config{Port: 8090, PhotosDir: photos}) {
		t.Errorf("not sharing: %+v, %v", got, err)
	}

	got, err = Decide(Config{ChannelsDir: "/Volumes/nas/.unterlumen-shared"}, Choice{PhotosDir: photos, Share: true}, lib, lib)
	if err != nil || got.ChannelsDir != "/Volumes/nas/.unterlumen-shared" {
		t.Errorf("sharing elsewhere is kept: %+v, %v", got, err)
	}
	if FindShared(photos) != "" {
		t.Error("keeping a share elsewhere made a shared folder in the photo folder")
	}

	got, err = Decide(Config{}, Choice{PhotosDir: photos, Share: true}, lib, lib)
	if err != nil || got.ChannelsDir != shared || FindShared(photos) == "" {
		t.Errorf("sharing: %+v, %v", got, err)
	}

	got, err = Decide(Config{ChannelsDir: shared}, Choice{PhotosDir: photos, LibDir: "/own"}, lib, lib)
	if err != nil || got.ChannelsDir != "/own" {
		t.Errorf("declining a shared folder that is there: %+v, %v", got, err)
	}
}

func TestShareRefusesTheTopOfADisk(t *testing.T) {
	if _, err := Share("/", t.TempDir()); !errors.Is(err, ErrShareAtDiskRoot) {
		t.Errorf("Share(\"/\") = %v, want ErrShareAtDiskRoot", err)
	}
	if !IsDiskRoot("/") || IsDiskRoot("/Volumes/nas/Bilder") || IsDiskRoot(t.TempDir()) {
		t.Error("IsDiskRoot misjudged a folder")
	}
}

func TestDecideKeepsASharedFolderElsewhereAtTheTopOfADisk(t *testing.T) {
	got, err := Decide(Config{ChannelsDir: "/Volumes/nas/Bilder/.unterlumen-shared"}, Choice{PhotosDir: "/", Share: true}, "/lib", "/lib")
	if err != nil || got.ChannelsDir != "/Volumes/nas/Bilder/.unterlumen-shared" {
		t.Errorf("Decide = %+v, %v; the share adopted from an older launcher must stay", got, err)
	}
}
