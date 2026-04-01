package templates_test

import (
	"fmt"
	"log"
	"testing"

	"github.com/glitchedgitz/grroxy/grx/templates"
)

func TestSetup(t *testing.T) {

	scenarios := []struct {
		data            map[string]any
		hook            string
		expextedResults int
	}{
		{
			hook: "proxy:request",
			data: map[string]any{
				"index": 123,
				"req": map[string]any{
					"ext":  ".pdf",
					"path": "testing/graphql/this",
				},
			},
			expextedResults: 2,
		},
		{
			hook: "proxy:request",
			data: map[string]any{
				"index": 123,
				"req": map[string]any{
					"ext":  ".gif",
					"path": "testing/this",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:request",
			data: map[string]any{
				"index": 123,
				"req": map[string]any{
					"ext":  ".jpg",
					"path": "testing/this",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:request",
			data: map[string]any{
				"index": 123,
				"req": map[string]any{
					"ext":  ".weird",
					"path": "testing/this",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:request",
			data: map[string]any{
				"index": 123,
				"req": map[string]any{
					"ext":  ".svg",
					"path": "testing/this",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:request",
			data: map[string]any{

				"index": 123,
				"req": map[string]any{
					"ext":  ".something",
					"path": "testing/this.something",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:request",
			data: map[string]any{

				"index": 123,
				"req": map[string]any{
					"ext":  ".somethingnew",
					"path": "testing/this.something",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:response",
			data: map[string]any{
				"index": 123,
				"resp": map[string]any{
					"mime": "image/svg",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:response",
			data: map[string]any{
				"index": 123,
				"resp": map[string]any{
					"mime": "audio/svg",
				},
			},
			expextedResults: 1,
		},
		// {
		// 	hook: "proxy:response",
		// 	data: map[string]any{
		// 		"index": 123,
		// 		"resp": map[string]any{
		// 			"mime": "xxxxxxxxxx/xxxxxxxxxx",
		// 		},
		// 	},
		// 	expextedResults: 1,
		// },
		// {
		// 	hook: "proxy:response",
		// 	data: map[string]any{
		// 		"index": 123,
		// 		"resp": map[string]any{
		// 			"mime": "22222222222/222222222222",
		// 		},
		// 	},
		// 	expextedResults: 1,
		// },
		{
			hook: "proxy:response",
			data: map[string]any{
				"index": 123,
				"resp": map[string]any{
					"mime": "video/ccc",
				},
			},
			expextedResults: 1,
		},
		{
			hook: "proxy:before_request",
			data: map[string]any{
				"index": 123,
				"host":  "https://asdfsdfasdupwork.com//",
				"req": map[string]any{
					"headers": map[string]any{
						"User-Agent": "Morzilla",
					},
				},
			},
			expextedResults: 1,
		},
	}

	// if os.Getenv("GRROXY_TEMPLATE_DIR") == "" {
	// 	t.Skip("GRROXY_TEMPLATE_DIR is not set; skipping template integration test")
	// }
	x := &templates.Templates{
		// TempalteDir: os.Getenv("GRROXY_TEMPLATE_DIR"),
	}

	x.Setup()

	for index, scenario := range scenarios {
		t.Run("TestSetup", func(t *testing.T) {
			log.Println("\n============================================ ", index)
			results, err := x.Run(scenario.data, scenario.hook)
			if err != nil {
				log.Println(err)
				t.Fatal(err)
			}

			for _, y := range results {
				fmt.Println("Got ---------------->", y)
			}

			if len(results) != scenario.expextedResults {
				t.Fatalf("expected: %v, got %v", scenario.expextedResults, results)
			}
		})
	}
}
