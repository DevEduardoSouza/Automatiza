package whatsapp

import (
	"context"
	"errors"
	"testing"

	"github.com/go-redis/redismock/v9"
	"go.uber.org/zap"
)

// Checagem executável do recorder: só a assinatura 420 incrementa; nil-safety
// total (recorder nil, redis nil) não panica — o caminho de envio nunca pode
// ser afetado por este side-effect.
func TestRestrictedChipRecorder_Note(t *testing.T) {
	ctx := context.Background()

	// Recorder nil e rdb nil: no-op sem pânico.
	var nilRec *RestrictedChipRecorder
	nilRec.Note(ctx, "session-a", errors.New("server returned error 420"))
	if NewRestrictedChipRecorder(nil, zap.NewNop()) != nil {
		t.Fatal("rdb nil deve desabilitar o recorder (nil), como o UploadPacer")
	}

	// Erro SEM a assinatura 420: nenhum comando Redis.
	db, mock := redismock.NewClientMock()
	rec := NewRestrictedChipRecorder(db, zap.NewNop())
	rec.Note(ctx, "session-a", errors.New("context deadline exceeded"))
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("erro sem 420 não pode tocar o Redis: %v", err)
	}

	// Erro COM a assinatura: pipeline HIncrBy+HSet+Expire×2.
	mock.ExpectHIncrBy(RestrictedChipsCountKey, "session-a", 1).SetVal(1)
	mock.Regexp().ExpectHSet(RestrictedChipsSeenKey, "session-a", `\d+`).SetVal(1)
	mock.ExpectExpire(RestrictedChipsCountKey, restrictedChipsTTL).SetVal(true)
	mock.ExpectExpire(RestrictedChipsSeenKey, restrictedChipsTTL).SetVal(true)
	rec.Note(ctx, "session-a", errors.New("WAHA error 500: ... 2 UNKNOWN: server returned error 420 ..."))
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("erro 420 deveria registrar no Redis: %v", err)
	}
}
