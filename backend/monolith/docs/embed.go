package docs

import "embed"

//go:embed swagger.json
var SwaggerJSON []byte

//go:embed index.html
var IndexHTML []byte

//go:embed swagger-ui/*
var StaticFS embed.FS
