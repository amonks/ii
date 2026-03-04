package markdown

import "testing"

type panicRenderer struct{}

func (panicRenderer) Render(string) (string, error) {
	panic("boom")
}

func TestSafeRender_RecoversFromRendererPanic(t *testing.T) {
	const renderWidth = 20

	rendererMu.Lock()
	prev, hadPrev := renderers[renderWidth]
	renderers[renderWidth] = panicRenderer{}
	rendererMu.Unlock()

	defer func() {
		rendererMu.Lock()
		if hadPrev {
			renderers[renderWidth] = prev
		} else {
			delete(renderers, renderWidth)
		}
		rendererMu.Unlock()
	}()

	out := SafeRender(renderWidth, 0, []byte("hello\n"))
	if string(out) != "hello" {
		t.Fatalf("expected fallback to original markdown, got %q", string(out))
	}
}
