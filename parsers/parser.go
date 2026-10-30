package parsers

import (
	"context"
	"fmt"

	"github.com/charmbracelet/log"

	"github.com/krau/SaveAny-Bot/parsers/js"
	"github.com/krau/SaveAny-Bot/parsers/native/kemono"
	"github.com/krau/SaveAny-Bot/parsers/native/twitter"
	"github.com/krau/SaveAny-Bot/parsers/parsers"
	"github.com/krau/SaveAny-Bot/pkg/parser"
)

func init() {
	parsers.Add(new(twitter.TwitterParser), new(kemono.KemonoParser))
}

var (
	ErrNoParserFound = fmt.Errorf("no parser found for the given URL")
)

func ParseWithContext(ctx context.Context, url string) (*parser.Item, error) {
	ch := make(chan *parser.Item, 1)
	errCh := make(chan error, 1)

	go func() {
		var lastErr error
		// Try every parser that can handle the URL; a failure of one parser
		// should not prevent the next one from acting as a fallback.
		for _, pser := range parsers.Get() {
			if !pser.CanHandle(url) {
				continue
			}
			item, err := pser.Parse(ctx, url)
			if err != nil {
				log.FromContext(ctx).Warnf("Parser %s failed to parse %s: %v, trying next parser if any", pserName(pser), url, err)
				lastErr = err
				continue
			}
			ch <- item
			return
		}
		if lastErr != nil {
			errCh <- lastErr
			return
		}
		errCh <- ErrNoParserFound
	}()

	select {
	case item := <-ch:
		return item, nil
	case err := <-errCh:
		return nil, err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func pserName(p parser.Parser) string {
	if c, ok := p.(parser.ConfigurableParser); ok {
		return c.Name()
	}
	return "unknown"
}

// CanHandle checks if any registered parser can handle the given URL and returns the parser if found.
func CanHandle(url string) (bool, parser.Parser) {
	for _, pser := range parsers.Get() {
		if pser.CanHandle(url) {
			return true, pser
		}
	}
	return false, nil
}

func LoadPlugins(ctx context.Context, dir string) error {
	return js.LoadPlugins(ctx, dir)
}

func AddPlugin(ctx context.Context, code string, name string) error {
	return js.AddPlugin(ctx, code, name)
}
