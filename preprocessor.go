package binding

import (
	"errors"
	"strings"
)

type Processor func(origin string) ([]string, error)

var processorMap = map[string]Processor{
	"split": func(origin string) ([]string, error) {
		return strings.Split(origin, ","), nil
	},
	"__testErr": func(origin string) ([]string, error) {
		return []string{}, errors.New("__testErr")
	},
}

func RegisterPreprocessor(name string, processor Processor) {
	processorMap[name] = processor
}

func getPreprocessor(name string) (Processor, bool) {
	p, ok := processorMap[name]
	return p, ok
}
