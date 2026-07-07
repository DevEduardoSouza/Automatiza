// Package videostream normaliza vídeos para o formato que o WhatsApp exige
// para de fato ENTREGAR (fan-out) a mídia.
//
// Um MP4 com o átomo "moov" no fim (não-faststart) — comum em vídeo baixado do
// WhatsApp e reenviado — é aceito pelo servidor do WhatsApp (ack=1) mas nunca
// distribuído aos aparelhos (nunca vira ack=2). O WAHA resolve isso rodando
// ffmpeg (convert) a cada envio; aqui remuxamos UMA vez, para poder mandar ao
// WAHA com convert:false e evitar transcodificar o mesmo vídeo por grupo em
// automações com muitos destinos (o WAHA é o gargalo do sistema).
package videostream

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ffmpegBin é o binário de transcodificação. Constante (não variável) para
// não abrir superfície de injeção — só os paths de arquivo temporário variam.
const ffmpegBin = "ffmpeg"

// Available reporta se o ffmpeg está disponível no PATH. O relay usa isto para
// decidir se pré-normaliza (Fase 2) ou cai no convert do WAHA (Fase 1).
func Available() bool {
	_, err := exec.LookPath(ffmpegBin)
	return err == nil
}

// EnsureFaststart remuxa o MP4 movendo o moov atom para o início (faststart),
// via stream copy — SEM re-encodar, então é rápido e sem perda de qualidade.
// Não altera codecs (vídeo do WhatsApp já é H.264/AAC). Retorna os bytes
// remuxados ou um erro quando o ffmpeg não está disponível/falha; nesse caso o
// chamador deve cair no convert do WAHA para não deixar de entregar.
func EnsureFaststart(ctx context.Context, data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("videostream: empty input")
	}

	dir, err := os.MkdirTemp("", "vstream-*")
	if err != nil {
		return nil, fmt.Errorf("videostream: mktemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	in := filepath.Join(dir, "in.mp4")
	out := filepath.Join(dir, "out.mp4")
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return nil, fmt.Errorf("videostream: write input: %w", err)
	}

	// -c copy: sem re-encode (rápido, lossless). -movflags +faststart: move o
	// moov para o início (faz uma segunda passada no arquivo de saída, por isso
	// precisa de saída em disco, não pipe). -f mp4: força o container.
	cmd := exec.CommandContext(ctx, ffmpegBin,
		"-nostdin", "-loglevel", "error", "-y",
		"-i", in,
		"-c", "copy",
		"-movflags", "+faststart",
		"-f", "mp4",
		out,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("videostream: ffmpeg failed: %w (%s)", err, stderr.String())
	}

	result, err := os.ReadFile(out)
	if err != nil {
		return nil, fmt.Errorf("videostream: read output: %w", err)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("videostream: empty output")
	}
	return result, nil
}
