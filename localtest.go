package psql

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

type processInfo struct {
	cmd   *exec.Cmd // allows tracking cmd.Process if needed
	ended bool
}

// LocalTestServer returns a backend that can be used for local tests, especially suitable for Go unit tests
// This requires having cockroach or apkg installed in order to run, and will start a local database
// with in-memory storage that will shutdown at the end of the tests. The database will always start in an
// empty state, and all data written to it will be lost once the execution completes.
func LocalTestServer() (*Backend, error) {
	// first, let's locate cockroach
	p, err := exec.LookPath("cockroach")
	if err != nil {
		// let's see if we got apkg
		if _, err2 := os.Stat("/pkg/main/dev-db.cockroach-bin.core/bin/cockroach"); err2 == nil {
			p = "/pkg/main/dev-db.cockroach-bin.core/bin/cockroach"
		} else {
			// cockroach not found
			return nil, fmt.Errorf("cockroach DB could not be found: %w", err)
		}
	}

	// prepare to run it
	cmd := exec.Command(p, "start-single-node", "--insecure", "--store=type=mem,size=50%", "--listen-addr=localhost:26259", "--sql-addr=localhost:26258", "--http-addr", "localhost:28081")

	cmd.Stdout = os.Stdout
	stderr, err := cmd.StderrPipe()
	if err != nil {
		// unlikely
		return nil, err
	}

	pi := &processInfo{
		cmd: cmd,
	}

	go readCockroachStdErr(stderr)

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start cockroach: %w", err)
	}

	go pi.wait()

	dsn := "postgresql://root@localhost:26258/defaultdb?sslmode=disable"

	// let's try to connect
	for i := 0; i < 120; i++ {
		err = attemptConnect(dsn)
		if err == nil {
			// success!
			return New(dsn)
		}
		// make sure process is still running
		if pi.ended {
			return nil, errors.New("cockroach db ended before we could connect to it")
		}

		time.Sleep(time.Second / 2)
	}

	return nil, fmt.Errorf("failed to connect to server: %w", err)
}

func readCockroachStdErr(pipe io.ReadCloser) {
	buf := bufio.NewReader(pipe)
	for {
		lin, err := buf.ReadString('\n')
		if err != nil {
			log.Printf("error: %s", err)
			return
		}

		lin = strings.TrimSpace(lin)

		if len(lin) == 0 {
			continue
		}

		switch lin[0] {
		case 'I', 'W':
			// Info or Warn: do nothin
		default:
			log.Printf("[cockroach] %s", lin)
		}
	}
}

func (pi *processInfo) wait() {
	pi.cmd.Wait()
	pi.ended = true
}

func attemptConnect(dsn string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	c, err := pgconn.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer c.Close(context.Background())

	return c.Ping(context.Background())
}
