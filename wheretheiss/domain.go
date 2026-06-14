package wheretheiss

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes wheretheiss as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/wheretheiss-cli/wheretheiss"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// wheretheiss:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone wheretheiss binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the wheretheiss driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "wheretheiss",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "wheretheiss",
			Short:  "Track the International Space Station from your terminal.",
			Long: `wheretheiss queries the Where The ISS At API to show the current
position of the International Space Station and its historical positions at any
Unix timestamp. No API key needed.`,
			Site: Host,
			Repo: "https://github.com/tamnd/wheretheiss-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// position: fetch the live ISS position.
	kit.Handle(app, kit.OpMeta{
		Name:    "position",
		Group:   "read",
		Single:  true,
		Summary: "Get the current ISS position",
		URIType: "position",
		Resolver: true,
	}, getPosition)

	// history: fetch ISS positions at given Unix timestamps.
	kit.Handle(app, kit.OpMeta{
		Name:    "history",
		Group:   "read",
		List:    true,
		Summary: "Get ISS positions at given Unix timestamps",
		URIType: "position",
		Args:    []kit.Arg{{Name: "timestamp", Help: "Unix timestamps (one or more)", Variadic: true}},
	}, getHistory)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type positionInput struct {
	Client *Client `kit:"inject"`
}

type historyInput struct {
	Timestamps []int64 `kit:"arg" help:"Unix timestamps"`
	Client     *Client `kit:"inject"`
}

// --- handlers ---

func getPosition(ctx context.Context, in positionInput, emit func(*Position) error) error {
	pos, err := in.Client.CurrentPosition(ctx)
	if err != nil {
		return mapErr(err)
	}
	return emit(pos)
}

func getHistory(ctx context.Context, in historyInput, emit func(*Position) error) error {
	positions, err := in.Client.HistoricalPositions(ctx, in.Timestamps)
	if err != nil {
		return mapErr(err)
	}
	for _, p := range positions {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns a resource reference into (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("empty wheretheiss reference")
	}
	return "position", input, nil
}

// Locate returns the live HTTPS URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	if uriType != "position" {
		return "", errs.Usage("wheretheiss has no resource type %q", uriType)
	}
	return BaseURL + "/v1/satellites/" + id, nil
}

func mapErr(err error) error {
	return err
}
