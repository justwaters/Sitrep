package stats

import "testing"

func TestInstallCommand(t *testing.T) {
	cases := []struct {
		pm, bin, want string
	}{
		{"apt-get", "ping", "sudo apt-get install -y iputils-ping"},
		{"dnf", "ping", "sudo dnf install -y iputils"},
		{"pacman", "ping", "sudo pacman -S --noconfirm iputils"},
		{"apt-get", "gh", "sudo apt-get install -y gh"},
		{"pacman", "gh", "sudo pacman -S --noconfirm github-cli"},
		{"unknown", "ping", ""},
		{"apt-get", "unknown-binary", ""},
	}
	for _, c := range cases {
		if got := InstallCommand(c.pm, c.bin); got != c.want {
			t.Errorf("InstallCommand(%q, %q) = %q, want %q", c.pm, c.bin, got, c.want)
		}
	}
}

func TestMissingEmpty(t *testing.T) {
	if got := Missing(nil); got != nil {
		t.Errorf("Missing(nil) = %v, want nil", got)
	}
}

func TestMissingUnknownBinary(t *testing.T) {
	// A synthetic check name with a binary that can never exist on any
	// test machine exercises the "reported missing" path deterministically.
	RequiredBinaries["__test_only_check__"] = []string{"__definitely_not_a_real_binary__"}
	defer delete(RequiredBinaries, "__test_only_check__")

	missing := Missing([]string{"__test_only_check__"})
	if len(missing) != 1 || missing[0] != "__definitely_not_a_real_binary__" {
		t.Errorf("Missing(...) = %v, want [__definitely_not_a_real_binary__]", missing)
	}
}
