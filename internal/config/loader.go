package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pyahu/cli/pkg/schema"
	"gopkg.in/yaml.v3"
)

const DefaultFileName = "pyahu.yaml"

const globalFileName = "pyahu.yaml"

var DefaultFileNames = []string{
	DefaultFileName,
	"pyahu.yml",
	filepath.Join(".pyahu", "stack.yaml"),
	filepath.Join(".pyahu", "stack.yml"),
}

type LoadedStack struct {
	Path        string
	Dir         string
	ProjectPath string
	GlobalPath  string
	Data        *schema.Stack
}

func Load(path string) (*LoadedStack, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return loadLayered("", abs)
}

func Find() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		for _, name := range DefaultFileNames {
			candidate := filepath.Join(dir, name)
			info, err := os.Stat(candidate)
			if err == nil {
				if info.IsDir() {
					return "", fmt.Errorf("project config %s is a directory", candidate)
				}
				return candidate, nil
			}
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("check project config %s: %w", candidate, err)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("no Pyahu stack file found (looked for pyahu.yaml, pyahu.yml, .pyahu/stack.yaml, .pyahu/stack.yml)")
}

func LoadFromFlag(path string) (*LoadedStack, error) {
	globalPath, hasGlobal, err := FindGlobal()
	if err != nil {
		return nil, err
	}
	if path == "" {
		found, err := Find()
		if err != nil {
			if hasGlobal {
				return loadLayered(globalPath, "")
			}
			return nil, err
		}
		path = found
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if hasGlobal && samePath(globalPath, abs) {
		globalPath = ""
	}
	return loadLayered(globalPath, abs)
}

func GlobalPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, globalFileName), nil
}

func FindGlobal() (string, bool, error) {
	path, err := GlobalPath()
	if err != nil {
		return "", false, nil
	}
	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return "", false, fmt.Errorf("global config %s is a directory", path)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", false, err
		}
		return abs, true, nil
	}
	if os.IsNotExist(err) {
		return "", false, nil
	}
	return "", false, fmt.Errorf("check global config %s: %w", path, err)
}

func loadLayered(globalPath string, projectPath string) (*LoadedStack, error) {
	var merged *yaml.Node
	if globalPath != "" {
		globalDoc, err := readStackDocument(globalPath)
		if err != nil {
			return nil, err
		}
		merged = globalDoc
	}
	if projectPath != "" {
		projectDoc, err := readStackDocument(projectPath)
		if err != nil {
			return nil, err
		}
		merged = mergeDocuments(merged, projectDoc)
	}
	if merged == nil || documentEmpty(merged) {
		return nil, fmt.Errorf("no Pyahu stack file found (looked for project pyahu.yaml and global config)")
	}

	stack, err := decodeStackDocument(merged)
	if err != nil {
		return nil, err
	}
	stack.SetDefaults()
	if err := stack.Validate(); err != nil {
		return nil, fmt.Errorf("invalid merged stack file: %w", err)
	}

	path := projectPath
	if path == "" {
		path = globalPath
	}
	return &LoadedStack{
		Path:        path,
		Dir:         filepath.Dir(path),
		ProjectPath: projectPath,
		GlobalPath:  globalPath,
		Data:        stack,
	}, nil
}

func readStackDocument(path string) (*yaml.Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read stack file %s: %w", path, err)
	}

	var partial schema.Stack
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&partial); err != nil {
		return nil, fmt.Errorf("parse stack file %s: %w", path, err)
	}

	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse stack file %s: %w", path, err)
	}
	return &document, nil
}

func decodeStackDocument(document *yaml.Node) (*schema.Stack, error) {
	data, err := yaml.Marshal(document)
	if err != nil {
		return nil, err
	}
	var stack schema.Stack
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&stack); err != nil {
		return nil, fmt.Errorf("parse merged stack file: %w", err)
	}
	return &stack, nil
}

func samePath(left string, right string) bool {
	leftAbs, leftErr := filepath.Abs(left)
	rightAbs, rightErr := filepath.Abs(right)
	if leftErr != nil || rightErr != nil {
		return left == right
	}
	return leftAbs == rightAbs
}

func WritePreset(path string, preset string, force bool) error {
	if path == "" {
		path = DefaultFileName
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("%s already exists; use --force to overwrite", path)
		}
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(path)), 0o755); err != nil && filepath.Dir(filepath.Clean(path)) != "." {
		return err
	}
	content, err := Preset(preset)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0o644)
}

func Preset(name string) (string, error) {
	switch name {
	case "", "minimal":
		return minimalPreset, nil
	case "platform":
		return platformPreset, nil
	default:
		return "", fmt.Errorf("unknown preset %q (supported: minimal, platform)", name)
	}
}

const minimalPreset = `apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: pyahu-local

services:
  postgres:
    enabled: true
    databases:
      - name: app
`

const platformPreset = `apiVersion: cli.pyahu.io/v1alpha1
kind: Stack
metadata:
  name: pyahu-local

cluster:
  runtime: k3d
  name: pyahu-local
  namespace: pyahu-local-dev
  servers: 1
  agents: 0

localTLS:
  enabled: true
  domains:
    - localhost
    - "*.localhost"

services:
  postgres:
    enabled: true
    version: "18.4"
    ports:
      primary: 5432
      read: 5433
    auth:
      username: pyahu
      password: pyahu_local
    instances: 1
    readReplicas: 0
    storage: 2Gi
    databases:
      - name: app
        owner: pyahu
      - name: zitadel
        owner: zitadel

  zitadel:
    enabled: true
    version: v4.15.2
    externalURL: https://zitadel.localhost
    databaseRef: postgres
    masterKey: MasterkeyNeedsToHave32Characters
    admin:
      username: admin@pyahu.local
      password: Password1!

  rabbitmq:
    enabled: true
    version: 4.3.2-management-alpine
    ports:
      amqp: 5672
    auth:
      username: pyahu
      password: pyahu_local
    replicas: 1
    storage: 2Gi
    management: true
    vhosts:
      - name: /
    users:
      - name: pyahu
        password: pyahu_local
        tags: administrator
        permissions:
          - vhost: /
            configure: .*
            write: .*
            read: .*

  kafka:
    enabled: true
    version: "4.3.0"
    ports:
      bootstrap: 9092
    replicas: 1
    storage: 4Gi
    topics:
      - name: app.events
        partitions: 1
        replicas: 1

  kafkaConnect:
    enabled: true
    image: quay.io/debezium/connect
    version: 3.5.2.Final
    ports:
      rest: 8083
    replicas: 1
    connectors: []

  kafkaUI:
    enabled: true
    image: ghcr.io/kafbat/kafka-ui
    version: v1.5.0
    replicas: 1
`
