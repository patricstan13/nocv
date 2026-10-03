package app

import (
	"fmt"

	. "example.com/shop/imports/helpers"
	_ "example.com/shop/imports/plugin"
	alias "example.com/shop/imports/repository"
	"example.com/shop/imports/service"
)

var (
	_ = fmt.Sprintf
	_ = HelperValue
	_ = alias.Value
	_ = service.Value
)
