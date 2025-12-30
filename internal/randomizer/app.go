package randomizer

import (
	"context"
	"math/rand/v2"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

var tracer = otel.Tracer("github.com/featherbread/randomizer/internal/randomizer")

// Store enables persistence for named groups of options.
type Store interface {
	// List returns the names of all available groups. If no groups have been
	// saved, it returns an empty list with a nil error.
	List(ctx context.Context) (groups []string, err error)

	// Get returns the list of options in the named group. If the group does not
	// exist, it returns an empty list with a nil error.
	Get(ctx context.Context, group string) (options []string, err error)

	// Put saves the provided options as a named group, overwriting any previous
	// group with that name.
	Put(ctx context.Context, group string, options []string) error

	// Delete ensures that the named group no longer exists, and indicates
	// whether the group existed prior to this deletion attempt.
	Delete(ctx context.Context, group string) (existed bool, err error)
}

// App represents a randomizer instance that can accept commands.
type App struct {
	name    string
	store   Store
	shuffle func([]string) // Overridden in tests for predictable behavior
}

func NewApp(name string, store Store) App {
	return App{
		name:    name,
		store:   store,
		shuffle: shuffle,
	}
}

func shuffle(options []string) {
	rand.Shuffle(len(options), func(i, j int) {
		options[i], options[j] = options[j], options[i]
	})
}

// Main is the entrypoint to the randomizer.
//
// All errors returned from Main are of type [Error], and support
// [Error.HelpText] for user-friendly formatting.
func (a App) Main(ctx context.Context, args []string) (Result, error) {
	ctx, span := tracer.Start(ctx, "Main")
	defer span.End()

	request, err := a.newRequest(ctx, args)
	if err != nil {
		span.RecordError(err)
		return Result{}, err
	}

	span.SetAttributes(attribute.String("randomizer.operation", request.Operation.String()))
	handler := appHandlers[request.Operation]
	return handler(a, request)
}

type appHandler func(App, request) (Result, error)

var appHandlers = map[operation]appHandler{
	showHelp:      App.showHelp,
	makeSelection: App.makeSelection,
	listGroups:    App.listGroups,
	showGroup:     App.showGroup,
	saveGroup:     App.saveGroup,
	deleteGroup:   App.deleteGroup,
}
