package app

import (
	"fmt"

	. "example.com/imports/helpers"
	_ "example.com/imports/plugin"
	alias "example.com/imports/repository"
	"example.com/imports/service"
)

var (
	_ = fmt.Sprintf
	_ = HelperValue
	_ = alias.Value
	_ = service.Value
)
