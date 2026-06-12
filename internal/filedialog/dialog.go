package filedialog

import (
	"errors"

	"github.com/sqweek/dialog"
)

var ErrCancelled = dialog.ErrCancelled

func OpenProjectDirectory() (string, error) {
	path, err := dialog.Directory().Title("打开项目").Browse()
	if errors.Is(err, dialog.ErrCancelled) {
		return "", ErrCancelled
	}
	return path, err
}
