package saiService

import (
	"context"
	"strings"
)

type Context struct {
	Configuration map[string]any
	Context       context.Context
}

func NewContext() *Context {
	return &Context{
		Configuration: map[string]any{},
		Context:       context.Background(),
	}
}

func (c *Context) SetValue(key string, value any) {
	c.Context = context.WithValue(context.Background(), key, value)
}

func (c *Context) GetConfig(path string, def any) any {
	steps := strings.Split(path, ".")
	configuration := c.Configuration

	if len(steps) == 0 {
		return def
	}

	for _, step := range steps {
		val, ok := configuration[step]

		if !ok {
			return def
		}

		switch val.(type) {
		case map[string]any:
			configuration = val.(map[string]any)
			break
		default:
			return val
		}
	}

	return configuration
}
