// seed loads the demo brand and the fixture corpus into Postgres.
//
// It writes one brand and one profile, then whatever labelled mentions the
// fixture file holds. It must never invent a mention: if the fixture file is
// not there it says so and seeds the brand alone, because a count on stage that
// came from nowhere is worse than a smaller count.
//
// -reset deletes this brand and nothing else. One Postgres is shared by every
// track in this repo, so dropping the database or the schema here would wipe
// six other people's work.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"brandpulse/internal/db"
	"brandpulse/internal/models"
)

// demoBrand is the shape of demo/brand.json. The brands table has no struct in
// internal/models, so its columns are spelled out here rather than invented
// there: internal/models is frozen and belongs to B1.
type demoBrand struct {
	Brand struct {
		ID                string `json:"id"`
		Name              string `json:"name"`
		Website           string `json:"website"`
		Plan              string `json:"plan"`
		WhatsappNumber    string `json:"whatsapp_number"`
		DailyCreditBudget int    `json:"daily_credit_budget"`
	} `json:"brand"`
	Profile models.BrandProfile `json:"profile"`
}

func main() {
	brandFile := flag.String("brand-file", "demo/brand.json", "the demo brand definition")
	fixtureFile := flag.String("fixtures", "fixtures/labelled/mentions.jsonl", "labelled mention corpus")
	reset := flag.Bool("reset", false, "delete the demo brand and everything hanging off it first")
	flag.Parse()

	if err := run(context.Background(), *brandFile, *fixtureFile, *reset, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, brandFile, fixtureFile string, reset bool, out io.Writer) error {
	brand, err := loadBrand(brandFile)
	if err != nil {
		return err
	}

	pool, err := db.Pool(ctx)
	if err != nil {
		return err
	}
	defer db.Close()

	if reset {
		if _, err := pool.Exec(ctx, `DELETE FROM brands WHERE id = $1`, brand.Brand.ID); err != nil {
			return fmt.Errorf("resetting %s: %w", brand.Brand.ID, err)
		}
		fmt.Fprintf(out, "reset: removed %s and every row that referenced it\n", brand.Brand.ID)
	}

	if err := seedBrand(ctx, pool, brand); err != nil {
		return err
	}
	fmt.Fprintf(out, "brand %s seeded with %d sources, fan-out cap is 8\n",
		brand.Brand.ID, len(brand.Profile.Sources))

	mentions, err := loadMentions(fixtureFile, brand.Brand.ID)
	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(out, "no corpus at %s, seeded the brand only. "+
			"The fixture recording is B2's and has not landed yet.\n", fixtureFile)
		return nil
	}
	if err != nil {
		return err
	}

	inserted, err := insertMentions(ctx, pool, mentions)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "corpus loaded: %d mentions read, %d new rows\n", len(mentions), inserted)
	return nil
}

func loadBrand(path string) (demoBrand, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return demoBrand{}, fmt.Errorf("reading the brand file: %w", err)
	}

	var brand demoBrand
	if err := json.Unmarshal(raw, &brand); err != nil {
		return demoBrand{}, fmt.Errorf("parsing %s: %w", path, err)
	}
	if brand.Brand.ID == "" {
		return demoBrand{}, fmt.Errorf("%s has no brand id", path)
	}
	if err := brand.Profile.Validate(); err != nil {
		return demoBrand{}, fmt.Errorf("%s: %w", path, err)
	}
	return brand, nil
}

// seedBrand upserts rather than inserts, so re-running without -reset keeps the
// corpus and the run history that make the demo look lived-in.
func seedBrand(ctx context.Context, pool *pgxpool.Pool, brand demoBrand) error {
	if _, err := pool.Exec(ctx, `
		INSERT INTO brands (id, name, website, plan, whatsapp_number, daily_credit_budget)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			website = EXCLUDED.website,
			plan = EXCLUDED.plan,
			whatsapp_number = EXCLUDED.whatsapp_number,
			daily_credit_budget = EXCLUDED.daily_credit_budget`,
		brand.Brand.ID, brand.Brand.Name, nullable(brand.Brand.Website),
		brand.Brand.Plan, nullable(brand.Brand.WhatsappNumber), brand.Brand.DailyCreditBudget,
	); err != nil {
		return fmt.Errorf("seeding the brand: %w", err)
	}

	columns := []struct {
		name  string
		value any
	}{
		{"keywords", brand.Profile.Keywords},
		{"hashtags", brand.Profile.Hashtags},
		{"products", brand.Profile.Products},
		{"competitors", brand.Profile.Competitors},
		{"sources", brand.Profile.Sources},
		{"negative_keywords", brand.Profile.NegativeKeywords},
		{"source_handles", brand.Profile.SourceHandles},
		{"voice", brand.Profile.Voice},
	}
	// pgx encodes a Go slice as a Postgres array, not as jsonb, so every one of
	// these columns needs an explicit marshal.
	encoded := make([]any, 0, len(columns))
	for _, column := range columns {
		value, err := json.Marshal(column.value)
		if err != nil {
			return fmt.Errorf("encoding profile.%s: %w", column.name, err)
		}
		encoded = append(encoded, value)
	}

	if _, err := pool.Exec(ctx, `
		INSERT INTO brand_profiles (
			brand_id, version, keywords, hashtags, products, competitors,
			sources, negative_keywords, source_handles, voice, confirmed_at
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10, now())
		ON CONFLICT (brand_id, version) DO UPDATE SET
			keywords = EXCLUDED.keywords,
			hashtags = EXCLUDED.hashtags,
			products = EXCLUDED.products,
			competitors = EXCLUDED.competitors,
			sources = EXCLUDED.sources,
			negative_keywords = EXCLUDED.negative_keywords,
			source_handles = EXCLUDED.source_handles,
			voice = EXCLUDED.voice,
			confirmed_at = now()`,
		append([]any{brand.Profile.BrandID, brand.Profile.Version}, encoded...)...,
	); err != nil {
		return fmt.Errorf("seeding the brand profile: %w", err)
	}
	return nil
}

// loadMentions reads the labelled corpus, one JSON object per line. The brand
// id on the line is overwritten with the demo brand's: the eval set is labelled
// for accuracy scoring, not for this brand, and a foreign key would reject it.
func loadMentions(path, brandID string) ([]models.Mention, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var mentions []models.Mention
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; scanner.Scan(); line++ {
		if len(scanner.Bytes()) == 0 {
			continue
		}
		var mention models.Mention
		if err := json.Unmarshal(scanner.Bytes(), &mention); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		mention.BrandID = brandID
		if err := mention.Validate(); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", path, line, err)
		}
		mentions = append(mentions, mention)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	return mentions, nil
}

func insertMentions(ctx context.Context, pool *pgxpool.Pool, mentions []models.Mention) (int, error) {
	batch := &pgx.Batch{}
	for _, mention := range mentions {
		engagement, err := json.Marshal(mention.Engagement)
		if err != nil {
			return 0, fmt.Errorf("mention %q engagement: %w", mention.ID, err)
		}
		raw, err := json.Marshal(mention.Raw)
		if err != nil {
			return 0, fmt.Errorf("mention %q raw: %w", mention.ID, err)
		}
		batch.Queue(`
			INSERT INTO mentions (
				id, brand_id, source, external_id, url, author, author_followers,
				text, lang, posted_at, engagement, rating, matched_keyword,
				content_hash, raw
			) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
			ON CONFLICT DO NOTHING`,
			mention.ID, mention.BrandID, mention.Source, mention.ExternalID,
			nullable(mention.URL), nullable(mention.Author), mention.AuthorFollowers,
			mention.Text, mention.Lang, mention.PostedAt, engagement, mention.Rating,
			nullable(mention.MatchedKeyword), mention.ContentHash, raw)
	}

	results := pool.SendBatch(ctx, batch)
	defer results.Close()

	inserted := 0
	for range mentions {
		tag, err := results.Exec()
		if err != nil {
			return 0, fmt.Errorf("loading the corpus: %w", err)
		}
		inserted += int(tag.RowsAffected())
	}
	return inserted, nil
}

// nullable keeps an empty optional column NULL rather than "". The dashboard
// renders a missing author differently from an author named nothing.
func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}
