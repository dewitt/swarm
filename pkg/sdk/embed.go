package sdk

import "embed"

//go:embed skills/*
//go:embed skills/*/*
var DefaultSkills embed.FS
