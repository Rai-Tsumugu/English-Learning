// Package license は LICENSES/ ディレクトリに必要なライセンス表記ファイルが
// 揃っているかを検証する (T31)。
package license

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// requiredFiles は LICENSES/ 直下に存在すべきファイル名 (大文字小文字区別なし)。
var requiredFiles = []string{
	"README.md",
	"THIRD_PARTY.md",
}

// Verify は licensesDir に必須ファイルが存在するか検証する。
// 欠落しているファイルがあれば、それらをまとめた error を返す。
// 呼び出し側は警告ログとして扱い、起動を妨げない運用を想定。
func Verify(licensesDir string) error {
	if licensesDir == "" {
		licensesDir = "./LICENSES"
	}
	info, err := os.Stat(licensesDir)
	if err != nil {
		return fmt.Errorf("licenses dir %q: %w", licensesDir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("licenses path %q is not a directory", licensesDir)
	}

	entries, err := os.ReadDir(licensesDir)
	if err != nil {
		return fmt.Errorf("read licenses dir: %w", err)
	}
	present := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		present[strings.ToLower(e.Name())] = true
	}

	var missing []string
	for _, name := range requiredFiles {
		if !present[strings.ToLower(name)] {
			missing = append(missing, filepath.Join(licensesDir, name))
		}
	}
	if len(missing) > 0 {
		return errors.New("missing required license files: " + strings.Join(missing, ", "))
	}
	return nil
}
