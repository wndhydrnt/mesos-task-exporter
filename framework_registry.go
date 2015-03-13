package main

import (
	"errors"
	"fmt"
	"sync"
)

type frameworkRegistry struct {
	mutex    *sync.Mutex
	registry map[string]Framework
}

func (fr *frameworkRegistry) Set(framework Framework) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	fr.registry[framework.Id] = framework
}

func (fr *frameworkRegistry) Get(id string) (Framework, error) {
	fr.mutex.Lock()
	defer fr.mutex.Unlock()

	fw, ok := fr.registry[id]
	if ok {
		return fw, nil
	} else {
		return Framework{}, errors.New(fmt.Sprintf("Unknown framwork '%s'", id))
	}
}

func NewFrameworkRegistry() *frameworkRegistry {
	return &frameworkRegistry{
		mutex:    &sync.Mutex{},
		registry: make(map[string]Framework),
	}
}
