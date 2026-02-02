package utils

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"text/template"
)

func AsTemplate(s string) (*template.Template, bool) {
	// Check if given string has template {{ }}
	re := regexp.MustCompile(`{{([\w.]+)}}`)

	subMatches := re.FindStringSubmatch(s)

	if len(subMatches) > 0 {
		promptTemplate := re.ReplaceAllString(s, "{{ .$1 }}")
		tmpl, err := template.New("prompt").Parse(promptTemplate)
		if err != nil {
			return nil, false
		}

		return tmpl, true
	}

	return nil, false
}

func ExecuteTemplate(t *template.Template, data map[string]any) (string, error) {
	var out bytes.Buffer
	err := t.Execute(&out, data)
	if err != nil {
		return "", err
	}
	return out.String(), nil
}

func TryAndParseAsTemplate(s string, data map[string]any) string {
	tmpl, ok := AsTemplate(s)
	if ok {
		vv, err := ExecuteTemplate(tmpl, data)
		if err != nil {
			return s
		}
		return vv
	}

	return s
}

func EnvironmentVariables() map[string]string {
	out := map[string]string{}

	envs := os.Environ()
	for _, env := range envs {
		envkv := strings.Split(env, "=")
		out[envkv[0]] = envkv[1]
	}

	return out
}
