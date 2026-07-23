package postgres

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"automatiza-go/internal/models"
	"automatiza-go/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.uber.org/zap"
)

// setupInstagramTestDB sobe um PostgreSQL efêmero e aplica as migrações REAIS
// 073 e 077 — não um schema reescrito à mão. É o que faz este teste provar o
// arquivo que vai para produção, e não uma cópia que pode divergir dele.
func setupInstagramTestDB(t *testing.T) (*DB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err, "failed to start postgres container")

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))

	// gen_random_uuid() vem do pgcrypto nas migrações 073/077.
	_, err = pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto;`)
	require.NoError(t, err)

	for _, name := range []string{"073_instagram_stories.up.sql", "077_instagram_story_hardening.up.sql"} {
		applyMigration(t, ctx, pool, name)
	}

	db := &DB{pool: pool, logger: zap.NewNop()}
	cleanup := func() {
		pool.Close()
		require.NoError(t, pgContainer.Terminate(ctx))
	}
	return db, cleanup
}

func applyMigration(t *testing.T, ctx context.Context, pool *pgxpool.Pool, name string) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "scripts", "db", "migrations", name)
	sql, err := os.ReadFile(path)
	require.NoError(t, err, "migração %s ilegível", name)
	_, err = pool.Exec(ctx, string(sql))
	require.NoError(t, err, "migração %s falhou", name)
}

const (
	igUserA = "11111111-1111-4111-8111-111111111111"
	igUserB = "22222222-2222-4222-8222-222222222222"
)

func skipIfShort(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// A 077 aplica sobre a 073 sem erro e cria tudo que o código espera.
func TestInstagramMigration077_AppliesOn073(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()

	checks := map[string]string{
		"coluna app_scoped_user_id": `SELECT 1 FROM information_schema.columns
			WHERE table_name='instagram_credentials' AND column_name='app_scoped_user_id'`,
		"coluna container_id": `SELECT 1 FROM information_schema.columns
			WHERE table_name='instagram_story_posts' AND column_name='container_id'`,
		"coluna processing_started_at": `SELECT 1 FROM information_schema.columns
			WHERE table_name='instagram_story_posts' AND column_name='processing_started_at'`,
		"índice único do evento": `SELECT 1 FROM pg_indexes
			WHERE indexname='uq_instagram_story_posts_source_event'`,
		"índice único app-scoped": `SELECT 1 FROM pg_indexes
			WHERE indexname='uq_instagram_credentials_app_scoped_user_id'`,
		"índice único ig_user_id": `SELECT 1 FROM pg_indexes
			WHERE indexname='uq_instagram_credentials_ig_user_id'`,
		"tabela de comprovantes": `SELECT 1 FROM information_schema.tables
			WHERE table_name='instagram_data_deletion_requests'`,
		"constraint de attempt_count": `SELECT 1 FROM pg_constraint
			WHERE conname='instagram_story_posts_attempt_count_nonnegative'`,
	}
	for name, q := range checks {
		var one int
		require.NoError(t, db.pool.QueryRow(ctx, q).Scan(&one), "faltou: %s", name)
	}
}

// O down devolve o schema ao estado da 073 — e SÓ isso: as tabelas da 073
// precisam continuar de pé.
func TestInstagramMigration077_DownLeaves073Intact(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()

	applyMigration(t, ctx, db.pool, "077_instagram_story_hardening.down.sql")

	var gone int
	err := db.pool.QueryRow(ctx, `SELECT 1 FROM information_schema.columns
		WHERE table_name='instagram_credentials' AND column_name='app_scoped_user_id'`).Scan(&gone)
	require.Error(t, err, "app_scoped_user_id deveria ter sido removida")

	// As tabelas da 073 permanecem.
	for _, table := range []string{"instagram_credentials", "instagram_story_posts"} {
		var one int
		require.NoError(t, db.pool.QueryRow(ctx,
			`SELECT 1 FROM information_schema.tables WHERE table_name=$1`, table).Scan(&one),
			"tabela %s da 073 não deveria ter sido removida", table)
	}
}

// Os índices únicos impedem que duas contas locais reivindiquem a MESMA conta
// do Instagram. Sem eles, dois clientes publicariam no mesmo perfil e a
// exclusão de dados apagaria a credencial errada.
func TestInstagramCredentials_UniqueAccountPerLocalUser(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramCredentialsRepository(db, zap.NewNop())
	exp := time.Now().Add(30 * 24 * time.Hour)

	require.NoError(t, repo.Save(ctx, igUserA, "{}", "APP-1", "IG-1", "loja", exp))

	// Mesmo app-scoped id em OUTRO usuário local.
	err := repo.Save(ctx, igUserB, "{}", "APP-1", "IG-2", "outra", exp)
	require.ErrorIs(t, err, repository.ErrInstagramAccountAlreadyConnected)

	// Mesmo id profissional em OUTRO usuário local.
	err = repo.Save(ctx, igUserB, "{}", "APP-2", "IG-1", "outra", exp)
	require.ErrorIs(t, err, repository.ErrInstagramAccountAlreadyConnected)

	// Reconectar a MESMA conta no MESMO usuário continua sendo um upsert válido.
	require.NoError(t, repo.Save(ctx, igUserA, "{novo}", "APP-1", "IG-1", "loja", exp))
}

// As linhas herdadas da 073 nascem com string vazia nos dois ids. O índice
// parcial existe justamente para elas não colidirem entre si.
func TestInstagramCredentials_EmptyIdentifiersDoNotCollide(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramCredentialsRepository(db, zap.NewNop())
	exp := time.Now().Add(30 * 24 * time.Hour)

	require.NoError(t, repo.Save(ctx, igUserA, "{}", "", "", "a", exp))
	require.NoError(t, repo.Save(ctx, igUserB, "{}", "", "", "b", exp),
		"duas linhas legadas sem identificadores deveriam coexistir")
}

func TestInstagramCredentials_GetAndDeleteByAppScopedID(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramCredentialsRepository(db, zap.NewNop())
	exp := time.Now().Add(30 * 24 * time.Hour)
	require.NoError(t, repo.Save(ctx, igUserA, "{}", "APP-1", "IG-1", "loja", exp))

	got, err := repo.GetByAppScopedUserID(ctx, "APP-1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, igUserA, got.UserID)

	// String vazia NÃO pode casar com as linhas legadas.
	empty, err := repo.GetByAppScopedUserID(ctx, "")
	require.NoError(t, err)
	require.Nil(t, empty)

	require.NoError(t, repo.DeleteByAppScopedUserID(ctx, "APP-1"))
	got, err = repo.Get(ctx, igUserA)
	require.NoError(t, err)
	require.Nil(t, got)
}

// O coração da idempotência: N chamadas concorrentes do MESMO evento criam UMA
// linha, e exatamente uma delas recebe created=true.
func TestInstagramStoryPosts_GetOrCreateIsAtomic(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramStoryPostRepository(db, zap.NewNop())

	const racers = 8
	var wg sync.WaitGroup
	var mu sync.Mutex
	createdCount := 0
	ids := map[string]bool{}

	for i := 0; i < racers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			row, created, err := repo.GetOrCreate(ctx, &models.InstagramStoryPost{
				UserID: igUserA, SourceGroup: "grupo@g.us", MessageID: "msg-1",
				MediaSource: models.InstagramMediaSourceImage,
				Status:      models.InstagramStoryStatusPending,
			})
			if err != nil {
				return
			}
			mu.Lock()
			if created {
				createdCount++
			}
			ids[row.ID] = true
			mu.Unlock()
		}()
	}
	wg.Wait()

	require.Equal(t, 1, createdCount, "exatamente uma chamada deveria ter criado a linha")
	require.Len(t, ids, 1, "todas as chamadas deveriam ver a MESMA linha")

	var total int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM instagram_story_posts`).Scan(&total))
	require.Equal(t, 1, total)
}

// Eventos diferentes (outra mensagem, outro grupo, outro dono) não colidem.
func TestInstagramStoryPosts_DistinctEventsDoNotCollide(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramStoryPostRepository(db, zap.NewNop())

	base := models.InstagramStoryPost{
		UserID: igUserA, SourceGroup: "grupo@g.us", MessageID: "msg-1",
		MediaSource: models.InstagramMediaSourceImage, Status: models.InstagramStoryStatusPending,
	}
	variants := []models.InstagramStoryPost{
		base,
		func() models.InstagramStoryPost { v := base; v.MessageID = "msg-2"; return v }(),
		func() models.InstagramStoryPost { v := base; v.SourceGroup = "outro@g.us"; return v }(),
		func() models.InstagramStoryPost { v := base; v.UserID = igUserB; return v }(),
	}
	for i := range variants {
		_, created, err := repo.GetOrCreate(ctx, &variants[i])
		require.NoError(t, err)
		require.True(t, created, "variante %d deveria criar linha nova", i)
	}

	var total int
	require.NoError(t, db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM instagram_story_posts`).Scan(&total))
	require.Equal(t, len(variants), total)
}

// O lease: um único reivindicante; lease fresco bloqueia; lease velho pode ser
// tomado (senão um worker morto travaria a linha para sempre).
func TestInstagramStoryPosts_LeaseLifecycle(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramStoryPostRepository(db, zap.NewNop())

	row, _, err := repo.GetOrCreate(ctx, &models.InstagramStoryPost{
		UserID: igUserA, SourceGroup: "grupo@g.us", MessageID: "msg-1",
		MediaSource: models.InstagramMediaSourceImage, Status: models.InstagramStoryStatusPending,
	})
	require.NoError(t, err)

	stale := time.Now().Add(-2 * time.Minute)

	claimed, lease1, err := repo.TryStartProcessing(ctx, row.ID, stale)
	require.NoError(t, err)
	require.True(t, claimed, "primeiro reivindicante deveria pegar o lease")

	claimed, _, err = repo.TryStartProcessing(ctx, row.ID, stale)
	require.NoError(t, err)
	require.False(t, claimed, "lease FRESCO não pode ser tomado por outro")

	// Lease velho: staleBefore no futuro simula o tempo tendo passado.
	claimed, lease2, err := repo.TryStartProcessing(ctx, row.ID, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.True(t, claimed, "lease velho deveria poder ser retomado")
	require.NotEqual(t, lease1, lease2, "cada reivindicação gera um token de lease novo")

	after, err := repo.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, 2, after.AttemptCount, "cada reivindicação incrementa a tentativa")

	// O dono ANTIGO não pode liberar o lease do dono atual — é o cenário do
	// worker zumbi que acorda depois do timeout do asynq.
	require.NoError(t, repo.ReleaseProcessing(ctx, row.ID, lease1))
	after, err = repo.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, after.ProcessingStartedAt, "lease do dono atual foi liberado por um zumbi")

	// O dono atual libera normalmente.
	require.NoError(t, repo.ReleaseProcessing(ctx, row.ID, lease2))
	after, err = repo.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.Nil(t, after.ProcessingStartedAt)
}

// Concorrência real no lease: só um vence.
func TestInstagramStoryPosts_TryStartProcessingHasSingleWinner(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramStoryPostRepository(db, zap.NewNop())

	row, _, err := repo.GetOrCreate(ctx, &models.InstagramStoryPost{
		UserID: igUserA, SourceGroup: "grupo@g.us", MessageID: "msg-1",
		MediaSource: models.InstagramMediaSourceImage, Status: models.InstagramStoryStatusPending,
	})
	require.NoError(t, err)

	stale := time.Now().Add(-2 * time.Minute)
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if ok, _, err := repo.TryStartProcessing(ctx, row.ID, stale); err == nil && ok {
				mu.Lock()
				winners++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 1, winners, "exatamente um worker deveria reivindicar o lease")
}

// MarkResult grava o desfecho e SEMPRE limpa o lease.
func TestInstagramStoryPosts_MarkResultClearsLease(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramStoryPostRepository(db, zap.NewNop())

	row, _, err := repo.GetOrCreate(ctx, &models.InstagramStoryPost{
		UserID: igUserA, SourceGroup: "grupo@g.us", MessageID: "msg-1",
		MediaSource: models.InstagramMediaSourceImage, Status: models.InstagramStoryStatusPending,
	})
	require.NoError(t, err)
	_, _, err = repo.TryStartProcessing(ctx, row.ID, time.Now().Add(-2*time.Minute))
	require.NoError(t, err)
	require.NoError(t, repo.SaveContainerID(ctx, row.ID, "container-1"))

	require.NoError(t, repo.MarkResult(ctx, row.ID,
		models.InstagramStoryStatusPublished, "media-1", "", "", true))

	after, err := repo.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, models.InstagramStoryStatusPublished, after.Status)
	require.Equal(t, "media-1", after.IGMediaID)
	require.Equal(t, "container-1", after.ContainerID)
	require.True(t, after.PublishedReconciled)
	require.Nil(t, after.ProcessingStartedAt, "lease deveria ter sido limpo")

	// Linha terminal não volta a ser reivindicável.
	claimed, _, err := repo.TryStartProcessing(ctx, row.ID, time.Now().Add(time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "linha terminal não pode ser reivindicada")

	// E um desfecho NÃO sobrescreve outro: o zumbi que acorda depois não pode
	// carimbar failed sobre uma linha já publicada.
	require.NoError(t, repo.MarkResult(ctx, row.ID,
		models.InstagramStoryStatusFailed, "", models.InstagramErrTransientExhausted, "x", false))
	after, err = repo.GetByID(ctx, row.ID)
	require.NoError(t, err)
	require.Equal(t, models.InstagramStoryStatusPublished, after.Status, "desfecho terminal foi sobrescrito")
	require.Equal(t, "media-1", after.IGMediaID)
}

// A retenção respeita o lote e a janela: nada fora do corte é tocado.
func TestInstagramStoryPosts_DeleteOlderThanRespectsBatchAndWindow(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	repo := NewInstagramStoryPostRepository(db, zap.NewNop())

	old := time.Now().Add(-60 * 24 * time.Hour)
	for i := 0; i < 5; i++ {
		_, err := db.pool.Exec(ctx, `
			INSERT INTO instagram_story_posts (user_id, source_group, message_id, media_source, status, created_at)
			VALUES ($1, 'g@g.us', $2, 'image', 'published', $3)`,
			igUserA, "antiga-"+string(rune('a'+i)), old)
		require.NoError(t, err)
	}
	// Uma recente, que não pode ser tocada.
	_, err := db.pool.Exec(ctx, `
		INSERT INTO instagram_story_posts (user_id, source_group, message_id, media_source, status)
		VALUES ($1, 'g@g.us', 'recente', 'image', 'published')`, igUserA)
	require.NoError(t, err)

	cutoff := time.Now().Add(-30 * 24 * time.Hour)

	n, err := repo.DeleteOlderThan(ctx, cutoff, 2)
	require.NoError(t, err)
	require.Equal(t, int64(2), n, "o lote deveria ser respeitado")

	n, err = repo.DeleteOlderThan(ctx, cutoff, 100)
	require.NoError(t, err)
	require.Equal(t, int64(3), n)

	var remaining int
	require.NoError(t, db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM instagram_story_posts`).Scan(&remaining))
	require.Equal(t, 1, remaining, "a linha recente deveria ter sobrevivido")
}

// A exclusão é TRANSACIONAL: comprovante e apagamento acontecem juntos, e a
// repetição do mesmo sujeito preserva o código original.
func TestInstagramDataDeletion_ProcessIsTransactional(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()

	credsRepo := NewInstagramCredentialsRepository(db, zap.NewNop())
	storyRepo := NewInstagramStoryPostRepository(db, zap.NewNop())
	delRepo := NewInstagramDataDeletionRepository(db, zap.NewNop())

	exp := time.Now().Add(30 * 24 * time.Hour)
	require.NoError(t, credsRepo.Save(ctx, igUserA, "{}", "APP-1", "IG-1", "loja", exp))
	_, _, err := storyRepo.GetOrCreate(ctx, &models.InstagramStoryPost{
		UserID: igUserA, SourceGroup: "g@g.us", MessageID: "m-1",
		MediaSource: models.InstagramMediaSourceImage, Status: models.InstagramStoryStatusPending,
	})
	require.NoError(t, err)
	// Outro usuário não pode ser afetado.
	require.NoError(t, credsRepo.Save(ctx, igUserB, "{}", "APP-2", "IG-2", "outra", exp))

	// Valores com o formato exato das colunas CHAR(64).
	const hash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	const code = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"

	receipt, err := delRepo.Process(ctx, "APP-1", hash, code, time.Now().Add(30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, code, receipt.ConfirmationCode)

	// Credencial e ledger do sujeito sumiram.
	got, err := credsRepo.Get(ctx, igUserA)
	require.NoError(t, err)
	require.Nil(t, got)
	var stories int
	require.NoError(t, db.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM instagram_story_posts WHERE user_id=$1`, igUserA).Scan(&stories))
	require.Zero(t, stories)

	// O outro usuário permanece intacto.
	other, err := credsRepo.Get(ctx, igUserB)
	require.NoError(t, err)
	require.NotNil(t, other)

	// Repetição do MESMO sujeito com outro candidato preserva o código original.
	again, err := delRepo.Process(ctx, "APP-1", hash,
		"1111111111111111111111111111111111111111111111111111111111111111",
		time.Now().Add(30*24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, code, again.ConfirmationCode, "o código do comprovante deve ser estável")

	// Consulta por código respeita a validade.
	found, err := delRepo.GetByConfirmationCode(ctx, code, time.Now())
	require.NoError(t, err)
	require.NotNil(t, found)

	expired, err := delRepo.GetByConfirmationCode(ctx, code, time.Now().Add(60*24*time.Hour))
	require.NoError(t, err)
	require.Nil(t, expired, "comprovante vencido deve responder como inexistente")
}

// Sujeito desconhecido ainda recebe comprovante — responder diferente revelaria
// se aquela conta existe aqui.
func TestInstagramDataDeletion_UnknownSubjectStillGetsReceipt(t *testing.T) {
	skipIfShort(t)
	db, cleanup := setupInstagramTestDB(t)
	defer cleanup()
	ctx := context.Background()
	delRepo := NewInstagramDataDeletionRepository(db, zap.NewNop())

	receipt, err := delRepo.Process(ctx, "NAO-EXISTE",
		"0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		"fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210",
		time.Now().Add(30*24*time.Hour))
	require.NoError(t, err)
	require.NotEmpty(t, receipt.ConfirmationCode)
	require.Equal(t, "completed", receipt.Status)
}

// A 073 não tinha índice único no evento e o dedupe do trigger era um SETNX
// fail-open de 1h: com Redis indisponível, duas entregas criavam duas linhas
// idênticas. Se existir um par assim, a 077 ABORTA no CREATE UNIQUE INDEX.
//
// Este teste fixa esse comportamento como intencional — falhar alto é melhor
// que deduplicar dados de cliente em silêncio — e serve de receita: se a
// migração abortar num ambiente, a query abaixo mostra exatamente o que limpar
// antes de reaplicar. (Em produção, verificado em 23/07/2026, as duas tabelas
// estavam vazias: 0 credenciais, 0 stories, 0 duplicatas.)
func TestInstagramMigration077_FailsLoudlyOnLegacyDuplicates(t *testing.T) {
	skipIfShort(t)
	ctx := context.Background()

	db, cleanup := setupInstagram073OnlyDB(t)
	defer cleanup()

	// Duas linhas do MESMO evento — impossível de criar hoje, possível na 073.
	for i := 0; i < 2; i++ {
		_, err := db.pool.Exec(ctx, `
			INSERT INTO instagram_story_posts (user_id, source_group, message_id, media_source, status)
			VALUES ($1, 'g@g.us', 'msg-duplicada', 'image', 'pending')`, igUserA)
		require.NoError(t, err)
	}

	// A query de pré-checagem que um operador deve rodar antes da 077.
	var dupes int
	require.NoError(t, db.pool.QueryRow(ctx, `
		SELECT count(*) FROM (
			SELECT 1 FROM instagram_story_posts
			WHERE message_id <> ''
			GROUP BY user_id, source_group, message_id HAVING count(*) > 1
		) d`).Scan(&dupes))
	require.Equal(t, 1, dupes, "a pré-checagem deveria enxergar o par duplicado")

	// E a migração precisa ABORTAR, não deduplicar silenciosamente.
	sql, err := os.ReadFile(filepath.Join("..", "..", "..", "scripts", "db", "migrations",
		"077_instagram_story_hardening.up.sql"))
	require.NoError(t, err)
	_, err = db.pool.Exec(ctx, string(sql))
	require.Error(t, err, "a 077 deveria falhar alto sobre dados duplicados legados")
}

// setupInstagram073OnlyDB sobe o banco com APENAS a 073 aplicada, para poder
// semear dados legados antes da 077.
func setupInstagram073OnlyDB(t *testing.T) (*DB, func()) {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	require.NoError(t, err)

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)
	require.NoError(t, pool.Ping(ctx))
	_, err = pool.Exec(ctx, `CREATE EXTENSION IF NOT EXISTS pgcrypto;`)
	require.NoError(t, err)
	applyMigration(t, ctx, pool, "073_instagram_stories.up.sql")

	db := &DB{pool: pool, logger: zap.NewNop()}
	return db, func() {
		pool.Close()
		require.NoError(t, pgContainer.Terminate(ctx))
	}
}
