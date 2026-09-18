package experiment

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"calcsockets/internal/shared/console"
	"calcsockets/internal/shared/metrics"
)

// Report reúne os resultados e os metadados necessários para reproduzi-los.
type Report struct {
	GeneratedAt    time.Time `json:"generatedAt"`
	Requests       int       `json:"requests"`
	Seed           int64     `json:"seed"`
	UDPTimeoutMS   int64     `json:"udpTimeoutMs"`
	UDPMaxAttempts int       `json:"udpMaxAttempts"`
	Results        []Result  `json:"results"`
}

// ConsoleTables devolve as tabelas das Partes 3 e 4 prontas para o terminal.
func (r Report) ConsoleTables(color bool) string {
	part3 := console.Table{Title: "Parte 3: UDP com perda simulada vs. TCP", Headers: console.SummaryHeaders}
	for _, result := range r.Results {
		part3.Rows = append(part3.Rows, console.SummaryRow(result.Scenario, result.Summary))
	}

	part4 := console.Table{Title: "Parte 4: tamanho médio das mensagens", Headers: sizeHeaders}
	part4.Rows = r.sizeRows()

	return part3.Render(color) + part4.Render(color)
}

// Markdown devolve o relatório em Markdown, com as tabelas e o detalhe por requisição.
func (r Report) Markdown() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Resultados do experimento\n\n")
	fmt.Fprintf(&b, "Gerado em %s. N = %d requisições por execução, seed %d, timeout UDP %d ms, até %d tentativas.\n",
		r.GeneratedAt.Format(time.RFC3339), r.Requests, r.Seed, r.UDPTimeoutMS, r.UDPMaxAttempts)
	b.WriteString("Todas as execuções usam a mesma sequência de requisições. Servidor e cliente rodam na mesma máquina (loopback).\n\n")

	b.WriteString("## Parte 3: UDP com perda simulada vs. TCP\n\n")
	rows := make([][]string, len(r.Results))
	for i, result := range r.Results {
		rows[i] = console.SummaryRow(result.Scenario, result.Summary)
	}
	writeMarkdownTable(&b, console.SummaryHeaders, rows)
	b.WriteString("\nRTT de cada requisição medido do primeiro envio até a resposta; no UDP inclui as esperas de retransmissão. ")
	b.WriteString("RTT médio e máximo consideram só as requisições respondidas.\n\n")

	b.WriteString("## Parte 4: tamanho médio das mensagens\n\n")
	writeMarkdownTable(&b, sizeHeaders, r.sizeRows())
	b.WriteString("\nBytes da mensagem serializada, sem enquadramento: o texto trafega com um '\\n' a mais e o protobuf com um prefixo varint de tamanho, de 1 byte para mensagens menores que 128 bytes.\n\n")

	b.WriteString("## Detalhe por requisição\n")
	for _, result := range r.Results {
		fmt.Fprintf(&b, "\n<details><summary>%s</summary>\n\n", result.Scenario)
		writeMarkdownTable(&b, []string{"seq", "requisição", "resposta", "tentativas", "RTT", "bytes req/resp"}, sampleRows(result.Summary))
		b.WriteString("\n</details>\n")
	}
	return b.String()
}

// Save grava experiment.json e experiment.md em dir e devolve os caminhos.
func (r Report) Save(dir string) ([]string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}

	files := map[string][]byte{
		filepath.Join(dir, "experiment.json"): data,
		filepath.Join(dir, "experiment.md"):   []byte(r.Markdown()),
	}
	var written []string
	for path, content := range files {
		if err := os.WriteFile(path, content, 0o644); err != nil {
			return written, err
		}
		written = append(written, path)
	}
	return written, nil
}

var sizeHeaders = []string{"Formato", "Requisição (B)", "Resposta (B)", "Total por troca (B)", "vs. texto"}

// sizeRows compara o texto da execução TCP com o protobuf, pois ambos usam o
// mesmo transporte e a mesma sequência de requisições.
func (r Report) sizeRows() [][]string {
	text, textOK := r.find(ProtocolTCP)
	binary, binaryOK := r.find(ProtocolProtobuf)
	if !textOK || !binaryOK {
		return nil
	}

	textTotal := text.AvgRequestBytes + text.AvgResponseBytes
	binaryTotal := binary.AvgRequestBytes + binary.AvgResponseBytes
	row := func(name string, s metrics.Summary, total float64, comparison string) []string {
		return []string{name, fmt.Sprintf("%.1f", s.AvgRequestBytes), fmt.Sprintf("%.1f", s.AvgResponseBytes),
			fmt.Sprintf("%.1f", total), comparison}
	}
	return [][]string{
		row("Texto (Parte 2)", text, textTotal, "referência"),
		row("Protobuf (Parte 4)", binary, binaryTotal, fmt.Sprintf("%+.1f%%", (binaryTotal/textTotal-1)*100)),
	}
}

func (r Report) find(protocol string) (metrics.Summary, bool) {
	for _, result := range r.Results {
		if result.Protocol == protocol {
			return result.Summary, true
		}
	}
	return metrics.Summary{}, false
}

func sampleRows(summary metrics.Summary) [][]string {
	rows := make([][]string, len(summary.Samples))
	for i, sample := range summary.Samples {
		reply, rtt := sample.Reply, console.FormatDuration(sample.RTT)
		if sample.Lost {
			reply, rtt = "perdida: "+sample.Failure, "-"
		}
		rows[i] = []string{
			fmt.Sprint(sample.Seq), "`" + sample.Request + "`", reply, fmt.Sprint(sample.Attempts), rtt,
			fmt.Sprintf("%d/%d", sample.RequestBytes, sample.ResponseBytes),
		}
	}
	return rows
}

func writeMarkdownTable(b *strings.Builder, headers []string, rows [][]string) {
	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat(" --- |", len(headers)) + "\n")
	for _, row := range rows {
		escaped := make([]string, len(row))
		for i, cell := range row {
			escaped[i] = strings.ReplaceAll(cell, "|", `\|`)
		}
		b.WriteString("| " + strings.Join(escaped, " | ") + " |\n")
	}
}
