package loaders

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// CardinalityData represents metric cardinality information
type CardinalityData struct {
	MetricName string
	Count      int64
}

// LabelsData represents metric labels information
type LabelsData struct {
	MetricName string
	Labels     []string
}

// JobMetricData represents complete metric data per job
type JobMetricData struct {
	Job              string
	MetricName       string
	Labels           []string
	Cardinality      int64
	LabelCardinality map[string]int64 // Per-label cardinality (label_name -> cardinality)
}

// LoadCardinalityReport loads metrics cardinality data from file
func LoadCardinalityReport(filename string) ([]CardinalityData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []CardinalityData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}

		count, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}

		data = append(data, CardinalityData{
			MetricName: strings.TrimSpace(parts[0]),
			Count:      count,
		})
	}

	return data, scanner.Err()
}

// LoadLabelsReport loads metrics labels data from file
func LoadLabelsReport(filename string) ([]LabelsData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []LabelsData
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}

		labelsStr := strings.TrimSpace(parts[1])
		// Remove quotes and split by comma
		labelsStr = strings.Trim(labelsStr, "\"")
		labels := strings.Split(labelsStr, ",")

		// Clean up labels
		var cleanLabels []string
		for _, label := range labels {
			cleanLabel := strings.TrimSpace(label)
			if cleanLabel != "" {
				cleanLabels = append(cleanLabels, cleanLabel)
			}
		}

		data = append(data, LabelsData{
			MetricName: strings.Trim(strings.TrimSpace(parts[0]), "\""),
			Labels:     cleanLabels,
		})
	}

	return data, scanner.Err()
}

// LoadJobMetricReport loads per-job metric data from file
func LoadJobMetricReport(filename string) ([]JobMetricData, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data []JobMetricData
	scanner := bufio.NewScanner(file)

	// Skip header line
	if scanner.Scan() {
		// JOB|METRIC_NAME|LABELS|CARDINALITY
	}

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) < 4 {
			continue
		}

		cardinality, err := strconv.ParseInt(strings.TrimSpace(parts[3]), 10, 64)
		if err != nil {
			continue
		}

		labelsStr := strings.TrimSpace(parts[2])
		labels := strings.Split(labelsStr, ",")

		// Clean up labels
		var cleanLabels []string
		for _, label := range labels {
			cleanLabel := strings.TrimSpace(label)
			if cleanLabel != "" {
				cleanLabels = append(cleanLabels, cleanLabel)
			}
		}

		// Parse per-label cardinality if present (5th column)
		var labelCardinality map[string]int64
		if len(parts) >= 5 && strings.TrimSpace(parts[4]) != "" {
			labelCardinality = make(map[string]int64)
			labelCardStr := strings.TrimSpace(parts[4])
			// Format: label1:count1,label2:count2,...
			labelCardParts := strings.Split(labelCardStr, ",")
			for _, part := range labelCardParts {
				kv := strings.Split(part, ":")
				if len(kv) == 2 {
					labelName := strings.TrimSpace(kv[0])
					count, err := strconv.ParseInt(strings.TrimSpace(kv[1]), 10, 64)
					if err == nil {
						labelCardinality[labelName] = count
					}
				}
			}
		}

		data = append(data, JobMetricData{
			Job:              strings.TrimSpace(parts[0]),
			MetricName:       strings.TrimSpace(parts[1]),
			Labels:           cleanLabels,
			Cardinality:      cardinality,
			LabelCardinality: labelCardinality,
		})
	}

	return data, scanner.Err()
}

// ConvertJobMetricToCardinality converts JobMetricData to CardinalityData
func ConvertJobMetricToCardinality(jobData []JobMetricData) []CardinalityData {
	var data []CardinalityData
	for _, jm := range jobData {
		data = append(data, CardinalityData{
			MetricName: jm.MetricName,
			Count:      jm.Cardinality,
		})
	}
	return data
}

// ConvertJobMetricToLabels converts JobMetricData to LabelsData
func ConvertJobMetricToLabels(jobData []JobMetricData) []LabelsData {
	var data []LabelsData
	for _, jm := range jobData {
		data = append(data, LabelsData{
			MetricName: jm.MetricName,
			Labels:     jm.Labels,
		})
	}
	return data
}
