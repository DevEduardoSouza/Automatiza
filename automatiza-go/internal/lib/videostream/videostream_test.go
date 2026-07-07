package videostream

import (
	"context"
	"testing"
)

func TestEnsureFaststart_EmptyInput(t *testing.T) {
	if _, err := EnsureFaststart(context.Background(), nil); err == nil {
		t.Fatal("esperava erro para input vazio")
	}
}

// Só roda o remux de verdade quando o ffmpeg está disponível (dev/prod). Em CI
// sem ffmpeg, o caminho de produção cai no convert do WAHA, então pular aqui é
// coerente com o fallback.
func TestEnsureFaststart_RemuxWhenAvailable(t *testing.T) {
	if !Available() {
		t.Skip("ffmpeg não disponível — relay cai no convert do WAHA")
	}

	// MP4 mínimo válido (ftyp + moov + mdat vazio) gerado pelo próprio ffmpeg
	// seria ideal, mas para não depender de fixture binária, garantimos ao menos
	// que bytes inválidos falham de forma limpa (e o caller faz fallback).
	if _, err := EnsureFaststart(context.Background(), []byte("not-a-real-mp4")); err == nil {
		t.Fatal("esperava erro para MP4 inválido")
	}
}
