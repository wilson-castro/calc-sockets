// Package config reúne os parâmetros de execução de todas as partes da atividade.
//
// A configuração é resolvida em três camadas, da menor para a maior precedência:
// valores padrão embutidos, o arquivo JSON (config.json na raiz, ou o caminho em
// CALC_CONFIG) e variáveis de ambiente CALC_*. Assim os binários funcionam sem
// nenhum argumento de linha de comando.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// DefaultPath é o arquivo lido quando CALC_CONFIG não está definida.
const DefaultPath = "config.json"

// Config agrupa as seções de configuração, espelhando a estrutura do config.json.
type Config struct {
	Network    Network    `json:"network"`
	Client     Client     `json:"client"`
	UDP        UDP        `json:"udp"`
	Experiment Experiment `json:"experiment"`
	Live       Live       `json:"live"`
	Log        Log        `json:"log"`
}

// Network define onde os servidores escutam e onde os clientes se conectam.
type Network struct {
	ListenHost string `json:"listenHost"`
	ServerHost string `json:"serverHost"`
	UDPPort    int    `json:"udpPort"`
	TCPPort    int    `json:"tcpPort"`
	ProtoPort  int    `json:"protoPort"`
}

// Client define a carga de trabalho e os limites de espera dos clientes.
type Client struct {
	Requests       int `json:"requests"`
	UDPTimeoutMS   int `json:"udpTimeoutMs"`
	UDPMaxAttempts int `json:"udpMaxAttempts"`
	DialTimeoutMS  int `json:"dialTimeoutMs"`
	IOTimeoutMS    int `json:"ioTimeoutMs"`
	// WorkloadSeed igual a zero sorteia uma sequência diferente a cada execução.
	WorkloadSeed int64 `json:"workloadSeed"`
}

// UDP define a perda simulada aplicada pelo servidor UDP.
type UDP struct {
	// LossRate é a fração, entre 0 e 1, dos datagramas recebidos que o servidor descarta.
	LossRate float64 `json:"lossRate"`
	// LossSeed igual a zero torna o sorteio das perdas diferente a cada execução.
	LossSeed int64 `json:"lossSeed"`
}

// Experiment define os cenários da Parte 3.
type Experiment struct {
	LossRates []float64 `json:"lossRates"`
	// Seed fixa carga de trabalho e perdas para que o experimento seja reprodutível.
	Seed       int64  `json:"seed"`
	ResultsDir string `json:"resultsDir"`
}

// Live define o painel em tempo real do console interativo.
type Live struct {
	// IntervalMS é a pausa entre requisições de cada cliente, para que o painel
	// possa ser acompanhado a olho.
	IntervalMS int `json:"intervalMs"`
	// LossRate é a perda UDP inicial do painel, ajustável com as teclas + e -.
	LossRate float64 `json:"lossRate"`
}

// Log define o nível mínimo e o modo de cor dos logs.
type Log struct {
	Level string `json:"level"`
	// Color aceita "auto" (cor apenas em terminal), "always" ou "never".
	Color string `json:"color"`
}

// Default devolve a configuração pedida no enunciado: N=20, timeout de 500 ms e 5 tentativas.
func Default() Config {
	return Config{
		Network: Network{ListenHost: "0.0.0.0", ServerHost: "127.0.0.1", UDPPort: 9000, TCPPort: 9001, ProtoPort: 9002},
		Client: Client{
			Requests:       20,
			UDPTimeoutMS:   500,
			UDPMaxAttempts: 5,
			DialTimeoutMS:  2000,
			IOTimeoutMS:    5000,
		},
		Experiment: Experiment{LossRates: []float64{0, 0.1, 0.3}, Seed: 2026, ResultsDir: "results"},
		Live:       Live{IntervalMS: 300, LossRate: 0.2},
		Log:        Log{Level: "info", Color: "auto"},
	}
}

// Load resolve a configuração final a partir dos padrões, do arquivo e do ambiente.
//
// A ausência do arquivo padrão não é erro. Um caminho explícito em CALC_CONFIG
// que não existe é erro, porque indica configuração digitada errada.
func Load() (Config, error) {
	cfg := Default()

	path, explicit := os.LookupEnv("CALC_CONFIG")
	if !explicit || path == "" {
		path = DefaultPath
	}
	if err := mergeFile(&cfg, path); err != nil {
		if !errors.Is(err, os.ErrNotExist) || explicit {
			return Config{}, err
		}
	}
	if err := applyEnv(&cfg); err != nil {
		return Config{}, err
	}
	return cfg, cfg.Validate()
}

// Validate rejeita combinações que impediriam as partes de funcionar.
func (c Config) Validate() error {
	var problems []error
	if c.Client.Requests <= 0 {
		problems = append(problems, errors.New("client.requests deve ser maior que zero"))
	}
	if c.Client.UDPTimeoutMS <= 0 || c.Client.DialTimeoutMS <= 0 || c.Client.IOTimeoutMS <= 0 {
		problems = append(problems, errors.New("timeouts do cliente devem ser maiores que zero"))
	}
	if c.Client.UDPMaxAttempts <= 0 {
		problems = append(problems, errors.New("client.udpMaxAttempts deve ser maior que zero"))
	}
	if c.Live.IntervalMS < 0 {
		problems = append(problems, errors.New("live.intervalMs não pode ser negativo"))
	}
	if !validRate(c.Live.LossRate) {
		problems = append(problems, fmt.Errorf("live.lossRate %v fora do intervalo [0, 1]", c.Live.LossRate))
	}
	if !validRate(c.UDP.LossRate) {
		problems = append(problems, fmt.Errorf("udp.lossRate %v fora do intervalo [0, 1]", c.UDP.LossRate))
	}
	for _, rate := range c.Experiment.LossRates {
		if !validRate(rate) {
			problems = append(problems, fmt.Errorf("experiment.lossRates contém %v, fora do intervalo [0, 1]", rate))
		}
	}
	return errors.Join(problems...)
}

// ListenAddress devolve o endereço de escuta de um servidor na porta informada.
func (c Config) ListenAddress(port int) string {
	return net.JoinHostPort(c.Network.ListenHost, strconv.Itoa(port))
}

// ServerAddress devolve o endereço que um cliente usa para alcançar a porta informada.
func (c Config) ServerAddress(port int) string {
	return net.JoinHostPort(c.Network.ServerHost, strconv.Itoa(port))
}

// UDPTimeout é a espera por resposta antes de retransmitir.
func (c Config) UDPTimeout() time.Duration { return milliseconds(c.Client.UDPTimeoutMS) }

// DialTimeout é o limite para estabelecer uma conexão TCP.
func (c Config) DialTimeout() time.Duration { return milliseconds(c.Client.DialTimeoutMS) }

// LiveInterval é a pausa entre requisições no painel em tempo real.
func (c Config) LiveInterval() time.Duration { return milliseconds(c.Live.IntervalMS) }

// IOTimeout é o limite de uma troca requisição-resposta sobre TCP.
func (c Config) IOTimeout() time.Duration { return milliseconds(c.Client.IOTimeoutMS) }

func mergeFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("arquivo de configuração %s inválido: %w", path, err)
	}
	return nil
}

// envOverride associa uma variável de ambiente ao campo que ela sobrescreve.
type envOverride struct {
	name  string
	apply func(cfg *Config, value string) error
}

var envOverrides = []envOverride{
	{"CALC_LISTEN_HOST", func(c *Config, v string) error { c.Network.ListenHost = v; return nil }},
	{"CALC_SERVER_HOST", func(c *Config, v string) error { c.Network.ServerHost = v; return nil }},
	{"CALC_UDP_PORT", intField(func(c *Config) *int { return &c.Network.UDPPort })},
	{"CALC_TCP_PORT", intField(func(c *Config) *int { return &c.Network.TCPPort })},
	{"CALC_PROTO_PORT", intField(func(c *Config) *int { return &c.Network.ProtoPort })},
	{"CALC_REQUESTS", intField(func(c *Config) *int { return &c.Client.Requests })},
	{"CALC_UDP_TIMEOUT_MS", intField(func(c *Config) *int { return &c.Client.UDPTimeoutMS })},
	{"CALC_UDP_MAX_ATTEMPTS", intField(func(c *Config) *int { return &c.Client.UDPMaxAttempts })},
	{"CALC_WORKLOAD_SEED", int64Field(func(c *Config) *int64 { return &c.Client.WorkloadSeed })},
	{"CALC_LOSS_RATE", floatField(func(c *Config) *float64 { return &c.UDP.LossRate })},
	{"CALC_LOSS_SEED", int64Field(func(c *Config) *int64 { return &c.UDP.LossSeed })},
	{"CALC_RESULTS_DIR", func(c *Config, v string) error { c.Experiment.ResultsDir = v; return nil }},
	{"CALC_LOG_LEVEL", func(c *Config, v string) error { c.Log.Level = v; return nil }},
	{"CALC_LOG_COLOR", func(c *Config, v string) error { c.Log.Color = v; return nil }},
}

// applyEnv ignora variáveis vazias porque o docker compose repassa variáveis
// não definidas no host como string vazia.
func applyEnv(cfg *Config) error {
	var problems []error
	for _, override := range envOverrides {
		value := os.Getenv(override.name)
		if value == "" {
			continue
		}
		if err := override.apply(cfg, value); err != nil {
			problems = append(problems, fmt.Errorf("%s=%q: %w", override.name, value, err))
		}
	}
	return errors.Join(problems...)
}

func intField(field func(*Config) *int) func(*Config, string) error {
	return func(c *Config, v string) error {
		parsed, err := strconv.Atoi(v)
		if err == nil {
			*field(c) = parsed
		}
		return err
	}
}

func int64Field(field func(*Config) *int64) func(*Config, string) error {
	return func(c *Config, v string) error {
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err == nil {
			*field(c) = parsed
		}
		return err
	}
}

func floatField(field func(*Config) *float64) func(*Config, string) error {
	return func(c *Config, v string) error {
		parsed, err := strconv.ParseFloat(v, 64)
		if err == nil {
			*field(c) = parsed
		}
		return err
	}
}

func validRate(rate float64) bool { return rate >= 0 && rate <= 1 }

func milliseconds(ms int) time.Duration { return time.Duration(ms) * time.Millisecond }
