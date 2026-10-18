package web

import "embed"

//go:embed templates/* static/* locales/*
var Files embed.FS
