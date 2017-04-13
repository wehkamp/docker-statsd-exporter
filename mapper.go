// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	identifierRE   = `[a-zA-Z_][a-zA-Z0-9_]+`

	labelLineRE  = regexp.MustCompile(`^(` + identifierRE + `)\s*=\s*"(.*)"$`)
	metricNameRE = regexp.MustCompile(`^` + identifierRE + `$`)
)

type metricMapping struct {
	regex  *regexp.Regexp
	labels prometheus.Labels
	ignore bool;
}

type metricMapper struct {
	mappings []metricMapping
	mutex    sync.Mutex
}

type configLoadStates int

const (
	SEARCHING configLoadStates = iota
	METRIC_DEFINITION
)

func (m *metricMapper) initFromString(fileContents string) error {
	lines := strings.Split(fileContents, "\n")
	numLines := len(lines)
	state := SEARCHING

	parsedMappings := []metricMapping{}
	currentMapping := metricMapping{labels: prometheus.Labels{}}
	for i, line := range lines {
		line := strings.TrimSpace(line)

		switch state {
		case SEARCHING:
			if line == "" {
				continue
			}
			re, err := regexp.Compile("^" + line + "$")
			if err != nil {
				return fmt.Errorf("Line %d: expected regex match line, got: %s on line %s", i, err, line)
			}
			currentMapping.regex = re
			state = METRIC_DEFINITION

		case METRIC_DEFINITION:
			if (i == numLines-1) && (line != "") {
				return fmt.Errorf("Line %d: missing terminating newline", i)
			}
			if line == "" {
				if !currentMapping.ignore {
					if len(currentMapping.labels) == 0 {
						return fmt.Errorf("Line %d: metric mapping didn't set any labels", i)
					}
					if _, ok := currentMapping.labels["name"]; !ok {
						return fmt.Errorf("Line %d: metric mapping didn't set a metric name", i)
					}
				}
				parsedMappings = append(parsedMappings, currentMapping)
				state = SEARCHING
				currentMapping = metricMapping{labels: prometheus.Labels{}}
				continue
			}
			if err := m.updateMapping(line, i, &currentMapping); err != nil {
				return err
			}
		default:
			panic("illegal state")
		}
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.mappings = parsedMappings

	mappingsCount.Set(float64(len(parsedMappings)))

	return nil
}

func (m *metricMapper) initFromFile(fileName string) error {
	mappingStr, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}
	return m.initFromString(string(mappingStr))
}

func (m *metricMapper) getMapping(statsdMetric string) (labels prometheus.Labels, present bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for _, mapping := range m.mappings {

		matches := mapping.regex.FindStringSubmatchIndex(statsdMetric)
		if len(matches) == 0 {
			continue
		}

		if mapping.ignore {
			return nil, true;
		}

		labels := prometheus.Labels{}
		for label, valueExpr := range mapping.labels {
			value := mapping.regex.ExpandString([]byte{}, valueExpr, statsdMetric, matches)
			labels[label] = string(value)
		}
		return labels, true
	}

	return nil, false
}

func (m *metricMapper) updateMapping(line string, i int, mapping *metricMapping) error {
	matches := labelLineRE.FindStringSubmatch(line)
	if len(matches) != 3 {
		return fmt.Errorf("Line %d: expected label mapping line, got: %s", i, line)
	}
	label, value := matches[1], matches[2]
	if label == "name" && !metricNameRE.MatchString(value) {
		return fmt.Errorf("Line %d: metric name '%s' doesn't match regex '%s'", i, value, metricNameRE)
	} else if label == "ignore" && value == "true" {
		(*mapping).ignore = true
        } else {
		(*mapping).labels[label] = value
	}
	return nil
}
