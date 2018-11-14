package main_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MagicalTux/gophp/core"
	"github.com/MagicalTux/gophp/core/tokenizer"
)

type phptest struct {
	f      *os.File
	reader *bufio.Reader
	output *bytes.Buffer
	name   string
	path   string

	p *core.Process
	g *core.Global

	t *testing.T
}

func (p *phptest) handlePart(part string, b *bytes.Buffer) error {
	switch part {
	case "TEST":
		testName := strings.TrimSpace(b.String())
		p.name += ": " + testName
		return nil
	case "FILE":
		// pass data to the engine
		t := tokenizer.NewLexer(b, p.path)
		ctx := core.NewContext(p.g)
		c := core.Compile(ctx, t)
		_, err := c.Run(ctx)
		return err
	case "EXPECT":
		// compare p.output with b
		out := bytes.TrimSpace(p.output.Bytes())
		exp := bytes.TrimSpace(b.Bytes())

		if bytes.Compare(out, exp) != 0 {
			return fmt.Errorf("output not as expected!\nExpected: %s\nGot: %s", exp, out)
		}
		return nil
	default:
		return fmt.Errorf("unhandled part type %s for test", part)
	}
}

func runTest(t *testing.T, path string) (p *phptest, err error) {
	p = &phptest{t: t, output: &bytes.Buffer{}, name: path, path: path}

	// read & parse test file
	p.f, err = os.Open(path)
	if err != nil {
		return
	}
	defer p.f.Close()
	p.reader = bufio.NewReader(p.f)

	var b *bytes.Buffer
	var part string

	// prepare env
	p.p = core.NewProcess()
	p.p.SetConstant("PHP_SAPI", "test")
	p.g = core.NewGlobal(context.Background(), p.p)
	p.g.SetOutput(p.output)

	for {
		lin, err := p.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return p, err
		}
		if strings.HasPrefix(lin, "--") {
			lin_trimmed := strings.TrimRight(lin, "\r\n")
			if strings.HasSuffix(lin_trimmed[2:], "--") {
				// start of a new thing?
				if b != nil {
					err := p.handlePart(part, b)
					if err != nil {
						return p, err
					}
				}
				thing := lin_trimmed[2 : len(lin_trimmed)-2]
				b = &bytes.Buffer{}
				part = thing
				continue
			}
		}

		if b == nil {
			return p, fmt.Errorf("malformed test file %s", path)
		}
		b.Write([]byte(lin))
	}
	if b != nil {
		err := p.handlePart(part, b)
		if err != nil {
			return p, err
		}
	}

	return p, nil
}

func TestPhp(t *testing.T) {
	// run all tests in "test"
	filepath.Walk("test", func(path string, info os.FileInfo, err error) error {
		if !info.Mode().IsRegular() {
			return err
		}
		if !strings.HasSuffix(path, ".phpt") {
			return err
		}

		p, err := runTest(t, path)
		if err != nil {
			t.Errorf("Error in %s: %s", p.name, err.Error())
		}
		return nil
	})
}