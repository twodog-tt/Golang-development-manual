package errgroup

import (
	"context"
	"errors"
	"testing"
)

func TestErrGroup_FirstError(t *testing.T) {
	g, ctx := WithContext(context.Background())

	g.Go(func() error {
		<-ctx.Done()
		return ctx.Err()
	})
	g.Go(func() error {
		return errors.New("boom")
	})

	if err := g.Wait(); err == nil || err.Error() != "boom" {
		t.Fatalf("want boom, got %v", err)
	}
}

func TestErrGroup_Success(t *testing.T) {
	g, _ := WithContext(context.Background())
	g.Go(func() error { return nil })
	g.Go(func() error { return nil })
	if err := g.Wait(); err != nil {
		t.Fatal(err)
	}
}
