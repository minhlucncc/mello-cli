package cmd

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/minhlucncc/mello-cli/internal/cache"
	"github.com/minhlucncc/mello-cli/internal/config"
	"github.com/minhlucncc/mello-cli/internal/mello"
	syncpkg "github.com/minhlucncc/mello-cli/internal/sync"
	"github.com/minhlucncc/mello-cli/internal/ui"
)

// cacheStore returns the cache for the current context: the workspace's
// .mello/cache when inside one, else a global cache under the config dir.
// Returns nil when caching is disabled (MELLO_NO_CACHE).
func cacheStore() *cache.Store {
	if os.Getenv("MELLO_NO_CACHE") != "" {
		return nil
	}
	if root, err := syncpkg.FindRoot("."); err == nil {
		return cache.New(filepath.Join(root, syncpkg.DirName, "cache"))
	}
	return cache.New(filepath.Join(config.Dir(), "cache"))
}

// cacheTTL is how long metadata stays fresh (default 5m, MELLO_CACHE_TTL secs).
func cacheTTL() time.Duration {
	if v := os.Getenv("MELLO_CACHE_TTL"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return time.Duration(n) * time.Second
		}
	}
	return 5 * time.Minute
}

// withCache returns the cached value for key, fetching and storing it on a miss.
func withCache[T any](key string, fetch func() (T, error)) (T, error) {
	store := cacheStore()
	var v T
	if store != nil && store.Get(key, cacheTTL(), &v) {
		return v, nil
	}
	v, err := fetch()
	if err != nil {
		return v, err
	}
	if store != nil {
		_ = store.Put(key, v)
	}
	return v, nil
}

func cachedWorkspaces(cx context.Context, cl *mello.Client) ([]mello.Workspace, error) {
	return withCache("workspaces", func() ([]mello.Workspace, error) { return cl.ListWorkspaces(cx) })
}

func cachedBoards(cx context.Context, cl *mello.Client, wsID string) ([]mello.Board, error) {
	return withCache("boards:"+wsID, func() ([]mello.Board, error) { return cl.ListBoards(cx, wsID) })
}

func cachedMembers(cx context.Context, cl *mello.Client, wsID string) ([]mello.Member, error) {
	return withCache("members:"+wsID, func() ([]mello.Member, error) { return cl.ListMembers(cx, wsID) })
}

func cachedColumns(cx context.Context, cl *mello.Client, boardID string) ([]mello.Column, error) {
	return withCache("columns:"+boardID, func() ([]mello.Column, error) { return cl.ListColumns(cx, boardID) })
}

// memberNames returns a userID→name map for a workspace (from cache).
func memberNames(cx context.Context, cl *mello.Client, wsID string) map[string]string {
	out := map[string]string{}
	members, err := cachedMembers(cx, cl, wsID)
	if err != nil {
		return out
	}
	for _, m := range members {
		if m.Name != "" {
			out[m.UserID] = m.Name
		}
	}
	return out
}

// invalidateCache drops cache entries (after a write that changes them).
func invalidateCache(keys ...string) {
	if s := cacheStore(); s != nil {
		for _, k := range keys {
			s.Delete(k)
		}
	}
}

func cacheCmd() *Command {
	return &Command{
		Name:  "cache",
		Short: "Manage the local metadata cache.",
		Subs: []*Command{
			{Name: "clear", Short: "Delete cached workspace/board/member data.", Run: cacheClear},
		},
	}
}

func cacheClear(args []string) error {
	fs, c := newFlags("cache clear")
	if err := parse(fs, c, args); err != nil {
		return err
	}
	s := cacheStore()
	if s == nil {
		ui.Successf("caching is disabled (MELLO_NO_CACHE)")
		return nil
	}
	if err := s.Clear(); err != nil {
		return err
	}
	ui.Successf("Cache cleared")
	return nil
}
