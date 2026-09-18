package live

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"calcsockets/internal/protocalc"
	"calcsockets/internal/shared/config"
	"calcsockets/internal/shared/logger"
	"calcsockets/internal/shared/random"
	"calcsockets/internal/shared/sequence"
	"calcsockets/internal/shared/workload"
	"calcsockets/internal/tcp"
	"calcsockets/internal/udp"
)

const (
	frameInterval = 200 * time.Millisecond
	lossStep      = 0.1
	eventCapacity = 200
	// pausePoll é a espera entre verificações enquanto o painel está pausado.
	pausePoll = 100 * time.Millisecond
)

// Sequências ANSI de controle de tela. A tela alternativa preserva o que estava
// no terminal antes, que reaparece quando o painel fecha.
const (
	enterAltScreen = "\033[?1049h\033[?25l"
	leaveAltScreen = "\033[?25h\033[?1049l"
	cursorHome     = "\033[H"
	clearLineEnd   = "\033[K"
	clearBelow     = "\033[J"
)

// Options configura o painel.
type Options struct {
	Config config.Config
	// Keys entrega as teclas digitadas. Canal fechado encerra o painel.
	Keys <-chan byte
	Out  io.Writer
	// Color liga cores no painel e nos eventos.
	Color bool
	// Size informa linhas e colunas do terminal a cada quadro, acompanhando redimensionamentos.
	Size func() (rows, cols int)
}

// lane é um protocolo em execução no painel: endereço do servidor, cliente e métricas.
type lane struct {
	view      ProtocolView
	address   string
	tracker   *Tracker
	exchanger sequence.Exchanger
	closer    io.Closer
}

// dashboard reúne o estado compartilhado entre clientes, teclado e renderização.
type dashboard struct {
	opts    Options
	events  *EventLog
	udp     *udp.Server
	lanes   []*lane
	paused  atomic.Bool
	started time.Time
}

// Run abre o painel e bloqueia até a tecla q, o fechamento de Keys ou o
// cancelamento de ctx. Sobe os três servidores em portas livres de 127.0.0.1 e os
// derruba ao sair.
func Run(ctx context.Context, opts Options) error {
	d := &dashboard{opts: opts, events: NewEventLog(eventCapacity), started: time.Now()}

	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	servers, err := d.startServers(runCtx)
	if err != nil {
		return err
	}
	defer servers.Wait()
	if err := d.connectClients(); err != nil {
		stop()
		return err
	}

	io.WriteString(opts.Out, enterAltScreen)
	defer io.WriteString(opts.Out, leaveAltScreen)

	var clients sync.WaitGroup
	for _, l := range d.lanes {
		clients.Add(1)
		go func() {
			defer clients.Done()
			d.drive(runCtx, l)
		}()
	}

	d.loop(runCtx)

	// Fechar os sockets destrava requisições em andamento; o último quadro avisa
	// que o painel está encerrando enquanto os clientes terminam.
	stop()
	d.draw(true)
	for _, l := range d.lanes {
		l.closer.Close()
	}
	clients.Wait()
	return nil
}

// loop processa teclas e redesenha o painel até o usuário sair.
func (d *dashboard) loop(ctx context.Context) {
	ticker := time.NewTicker(frameInterval)
	defer ticker.Stop()

	d.draw(false)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.draw(false)
		case key, open := <-d.opts.Keys:
			if !open || d.handleKey(key) {
				return
			}
			d.draw(false)
		}
	}
}

// handleKey aplica uma tecla e informa se o usuário pediu para sair.
func (d *dashboard) handleKey(key byte) (quit bool) {
	switch key {
	case 'q', 'Q', 27: // 27 = Esc
		return true
	case '+', '=':
		d.udp.SetLossRate(d.udp.LossRate() + lossStep)
	case '-', '_':
		d.udp.SetLossRate(d.udp.LossRate() - lossStep)
	case 'p', 'P', ' ':
		d.paused.Store(!d.paused.Load())
	case 'r', 'R':
		for _, l := range d.lanes {
			l.tracker.Reset()
		}
		d.started = time.Now()
	}
	return false
}

// drive gera carga contínua para um protocolo: uma requisição por vez, com a
// pausa configurada entre elas, como o cliente da atividade faz.
func (d *dashboard) drive(ctx context.Context, l *lane) {
	rng := random.New(0)
	for seq := uint64(0); ctx.Err() == nil; {
		if d.paused.Load() {
			sleep(ctx, pausePoll)
			continue
		}
		sample := l.exchanger.Exchange(workload.Next(rng, seq))
		if ctx.Err() != nil {
			return // requisição interrompida pelo encerramento; não entra nas métricas
		}
		l.tracker.Record(sample)
		seq++
		sleep(ctx, d.opts.Config.LiveInterval())
	}
}

func (d *dashboard) draw(stopping bool) {
	rows, cols := d.opts.Size()
	frame := Frame{
		Uptime:   time.Since(d.started),
		LossRate: d.udp.LossRate(),
		Interval: d.opts.Config.LiveInterval(),
		Paused:   d.paused.Load(),
		Stopping: stopping,
		Events:   d.events.Last(rows),
	}
	for _, l := range d.lanes {
		view := l.view
		view.Snapshot = l.tracker.Snapshot()
		frame.Protocols = append(frame.Protocols, view)
	}

	lines := Render(frame, cols, rows-1, d.opts.Color)
	io.WriteString(d.opts.Out, cursorHome+strings.Join(lines, clearLineEnd+"\n")+clearLineEnd+clearBelow)
}

// logger cria um logger que escreve no feed de eventos do painel.
func (d *dashboard) logger(component string, level slog.Level) *slog.Logger {
	return logger.New(component, d.events, logger.Options{Level: level, Color: d.opts.Color})
}

// runningServers acompanha as goroutines Serve para que Run espere o encerramento.
type runningServers struct{ sync.WaitGroup }

func (d *dashboard) startServers(ctx context.Context) (*runningServers, error) {
	cfg := d.opts.Config
	udpServer, err := udp.Listen("127.0.0.1:0",
		udp.ServerOptions{LossRate: cfg.Live.LossRate, LossSeed: cfg.UDP.LossSeed}, d.logger("udp-server", slog.LevelInfo))
	if err != nil {
		return nil, err
	}
	tcpServer, err := tcp.Listen("127.0.0.1:0", d.logger("tcp-server", slog.LevelInfo))
	if err != nil {
		udpServer.Close()
		return nil, err
	}
	protoServer, err := protocalc.Listen("127.0.0.1:0", d.logger("proto-server", slog.LevelInfo))
	if err != nil {
		udpServer.Close()
		tcpServer.Close()
		return nil, err
	}
	d.udp = udpServer

	running := &runningServers{}
	for _, server := range []interface{ Serve(context.Context) error }{udpServer, tcpServer, protoServer} {
		running.Add(1)
		go func() {
			defer running.Done()
			server.Serve(ctx)
		}()
	}

	d.lanes = []*lane{
		{view: ProtocolView{Name: "UDP", Color: logger.Blue}, address: udpServer.Addr(), tracker: &Tracker{}},
		{view: ProtocolView{Name: "TCP", Color: logger.Cyan}, address: tcpServer.Addr(), tracker: &Tracker{}},
		{view: ProtocolView{Name: "Protobuf", Color: logger.Magenta}, address: protoServer.Addr(), tracker: &Tracker{}},
	}
	return running, nil
}

func (d *dashboard) connectClients() error {
	cfg := d.opts.Config
	clientLog := func(component string) *slog.Logger { return d.logger(component, slog.LevelWarn) }

	udpClient, err := udp.Dial(d.lanes[0].address,
		udp.ClientOptions{Timeout: cfg.UDPTimeout(), MaxAttempts: cfg.Client.UDPMaxAttempts}, clientLog("udp-client"))
	if err != nil {
		return err
	}
	tcpClient, err := tcp.Dial(d.lanes[1].address,
		tcp.ClientOptions{DialTimeout: cfg.DialTimeout(), IOTimeout: cfg.IOTimeout()}, clientLog("tcp-client"))
	if err != nil {
		udpClient.Close()
		return err
	}
	protoClient, err := protocalc.Dial(d.lanes[2].address,
		protocalc.ClientOptions{DialTimeout: cfg.DialTimeout(), IOTimeout: cfg.IOTimeout()}, clientLog("proto-client"))
	if err != nil {
		udpClient.Close()
		tcpClient.Close()
		return err
	}

	d.lanes[0].exchanger, d.lanes[0].closer = udpClient, udpClient
	d.lanes[1].exchanger, d.lanes[1].closer = tcpClient, tcpClient
	d.lanes[2].exchanger, d.lanes[2].closer = protoClient, protoClient
	return nil
}

// sleep espera d ou até ctx ser cancelado, o que vier primeiro.
func sleep(ctx context.Context, d time.Duration) {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}
