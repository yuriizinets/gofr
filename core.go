package ssc

import (
	"io"
	"log"
	"net/http"
	"reflect"
	"sync"
	"time"
)

// Component Store Lifecycle is a temporary storage for components processing
// Will be cleared in the end of lifecycle
var csl = map[Page][]Component{}
var csllock = &sync.Mutex{}

// RegisterComponent is used while defining components in the Init() section
func RegisterComponent(p Page, c Component) Component {
	// Init csl store
	csllock.Lock()
	if _, ok := csl[p]; !ok {
		csl[p] = []Component{}
	}
	csllock.Unlock()
	// Save type to SSA store
	csssalock.Lock()
	if _, ok := csssa[reflect.ValueOf(c).Elem().Type().Name()]; !ok {
		// Extract component type
		ctype := reflect.ValueOf(c).Elem().Type()
		// Save to store
		csssa[reflect.ValueOf(c).Elem().Type().Name()] = ctype
	}
	csssalock.Unlock()
	// Save component to lifecycle store
	csllock.Lock()
	csl[p] = append(csl[p], c)
	csllock.Unlock()
	// Trigger component init
	if c, ok := c.(ImplementsNestedInit); ok {
		c.Init(p)
	}
	// Return component for external assignment
	return c
}

// Alias for RegisterComponent
var RegC = RegisterComponent

// RenderPage is a main entrypoint of rendering. Responsible for rendering and components lifecycle
func RenderPage(w io.Writer, p Page) {
	// Async specific state
	var wg sync.WaitGroup
	var err = make(chan error, 1000)
	// Trigger init
	if p, ok := p.(ImplementsInit); ok {
		st := time.Now()
		p.Init()
		et := time.Since(st)
		if BENCH_LOWLEVEL {
			log.Println("Init time", reflect.TypeOf(p), et)
		}
	}
	// Trigger async in goroutines
	csllock.Lock()
	for _, component := range csl[p] {
		if component, ok := component.(ImplementsAsync); ok {
			wg.Add(1)
			go func(wg *sync.WaitGroup, err chan error, c ImplementsAsync) {
				defer wg.Done()
				st := time.Now()
				_err := c.Async()
				et := time.Since(st)
				if BENCH_LOWLEVEL {
					log.Println("Async time", reflect.TypeOf(component), et)
				}
				if _err != nil {
					err <- _err
				}
			}(&wg, err, component)
		}
	}
	csllock.Unlock()
	// Wait for async completion
	wg.Wait()
	// Trigger aftersync
	csllock.Lock()
	for _, component := range csl[p] {
		if component, ok := component.(ImplementsAfterAsync); ok {
			st := time.Now()
			component.AfterAsync()
			et := time.Since(st)
			if BENCH_LOWLEVEL {
				log.Println("AfterAsync time", reflect.TypeOf(component), et)
			}
		}
	}
	csllock.Unlock()
	// Clear components store (not needed more)
	csllock.Lock()
	delete(csl, p)
	csllock.Unlock()
	// Execute template
	st := time.Now()
	terr := p.Template().Execute(w, reflect.ValueOf(p).Elem())
	et := time.Since(st)
	if BENCH_LOWLEVEL {
		log.Println("Execute time", et)
	}
	if terr != nil {
		panic(terr)
	}
}

// PageHandlerFactory is a factory for building Page handler.
// Simple wrapper around RenderPage with context setting.
// Good for defining own project-level handler.
// Example of usage:
// func handle(p ssc.Page) http.HandlerFunc {
//     return func(rw http.ResponseWriter, r *http.Request) {
// 	       ssc.PageHandlerFactory(p, map[string]interface{}{
//	           "internal:rw": rw,
//             "internal:r": r,
//         })(rw, r)
//     }
// }
func PageHandlerFactory(p Page, context map[string]interface{}) http.HandlerFunc {
	// Set context
	for k, v := range context {
		SetContext(p, k, v)
	}
	// Return handler
	return func(rw http.ResponseWriter, r *http.Request) {
		// Render page
		RenderPage(rw, p)
		// Clear context
		DelContext(p, "")
	}
}