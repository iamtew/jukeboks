package ytmd

import (
	"strings"
	"testing"
)

func TestBrowseIDForPlaylist(t *testing.T) {
	if got := browseIDForPlaylist("PLabc"); got != "VLPLabc" {
		t.Fatalf("browseIDForPlaylist(PLabc) = %q, want VLPLabc", got)
	}
	if got := browseIDForPlaylist("OLAK5uy_test"); got != "VLOLAK5uy_test" {
		t.Fatalf("browseIDForPlaylist(OLAK) = %q, want VLOLAK5uy_test", got)
	}
	if got := browseIDForPlaylist("VLPLabc"); got != "VLPLabc" {
		t.Fatalf("browseIDForPlaylist(VLPLabc) = %q, want VLPLabc", got)
	}
}

func TestParseInnertubeBrowseFixture(t *testing.T) {
	payload := map[string]any{
		"contents": map[string]any{
			"singleColumnBrowseResultsRenderer": map[string]any{
				"tabs": []any{
					map[string]any{
						"tabRenderer": map[string]any{
							"content": map[string]any{
								"sectionListRenderer": map[string]any{
									"contents": []any{
										map[string]any{
											"musicPlaylistShelfRenderer": map[string]any{
												"contents": []any{
													map[string]any{
														"musicResponsiveListItemRenderer": map[string]any{
															"flexColumns": []any{
																map[string]any{
																	"musicResponsiveListItemFlexColumnRenderer": map[string]any{
																		"text": map[string]any{
																			"runs": []any{map[string]any{
																				"text": "Album Track",
																				"navigationEndpoint": map[string]any{
																					"watchEndpoint": map[string]any{"videoId": "abc12345678"},
																				},
																			}},
																		},
																	},
																},
																map[string]any{
																	"musicResponsiveListItemFlexColumnRenderer": map[string]any{
																		"text": map[string]any{
																			"runs": []any{map[string]any{"text": "Album Artist"}},
																		},
																	},
																},
															},
															"fixedColumns": []any{
																map[string]any{
																	"musicResponsiveListItemFixedColumnRenderer": map[string]any{
																		"text": map[string]any{
																			"runs": []any{map[string]any{"text": "3:21"}},
																		},
																	},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
		"header": map[string]any{
			"musicDetailHeaderRenderer": map[string]any{
				"title": map[string]any{
					"runs": []any{map[string]any{"text": "Public Album"}},
				},
			},
		},
	}

	lookup, ok := parsePlaylistPayload("OLAK5uy_test", payload)
	if !ok {
		t.Fatal("parsePlaylistPayload() ok = false")
	}
	if lookup.Name != "Public Album" {
		t.Fatalf("name = %q, want Public Album", lookup.Name)
	}
	if len(lookup.Tracks) != 1 || lookup.Tracks[0].VideoID != "abc12345678" {
		t.Fatalf("tracks = %#v", lookup.Tracks)
	}
}

func TestInnertubeClientVersionShape(t *testing.T) {
	version := innertubeClientVersion()
	if !strings.HasPrefix(version, "1.") || !strings.HasSuffix(version, ".01.00") {
		t.Fatalf("innertubeClientVersion() = %q, want 1.YYYYMMDD.01.00", version)
	}
}
