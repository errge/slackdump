package man

import (
	_ "embed"

	"github.com/rusq/slackdump/v2/cmd/slackdump/internal/golang/base"
)

//go:embed assets/changelog.md
var mdWhatsnew string

var WhatsNew = &base.Command{
	UsageLine: "whatsnew",
	Short:     "what's new in this version",
	Long:      base.Render(mdWhatsnew),
}