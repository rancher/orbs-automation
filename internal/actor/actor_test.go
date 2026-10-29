package actor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gitConfig(m map[string]string) ConfigFunc {
	return func(_ context.Context, key string) (string, error) { return m[key], nil }
}

func userID(id int64) UserIDFunc {
	return func(context.Context, string) (int64, error) { return id, nil }
}

func TestBot(t *testing.T) {
	t.Run("name is the login and email carries the numeric id", func(t *testing.T) {
		got, err := Bot(context.Background(), "rancher-scc-bot[bot]", userID(12345))

		require.NoError(t, err)
		assert.Equal(t, Identity{
			Name:  "rancher-scc-bot[bot]",
			Email: "12345+rancher-scc-bot[bot]@users.noreply.github.com",
		}, got)
	})

	t.Run("asks for the login it was given", func(t *testing.T) {
		var asked string
		_, err := Bot(context.Background(), "gero-bot[bot]", func(_ context.Context, login string) (int64, error) {
			asked = login
			return 1, nil
		})

		require.NoError(t, err)
		assert.Equal(t, "gero-bot[bot]", asked)
	})

	t.Run("a failed lookup is an error naming the login", func(t *testing.T) {
		boom := errors.New("boom")
		_, err := Bot(context.Background(), "gero-bot[bot]", func(context.Context, string) (int64, error) { return 0, boom })

		require.ErrorIs(t, err, boom)
		assert.Contains(t, err.Error(), "gero-bot[bot]")
	})

	t.Run("an empty login is an error naming GERO_BOT_USER", func(t *testing.T) {
		_, err := Bot(context.Background(), "", userID(1))

		require.Error(t, err)
		assert.Contains(t, err.Error(), "GERO_BOT_USER")
	})
}

func TestWorkstation(t *testing.T) {
	t.Run("reads user.name and user.email", func(t *testing.T) {
		got, err := Workstation(context.Background(), gitConfig(map[string]string{
			"user.name": "Dan Pock", "user.email": "dan@example.com",
		}))

		require.NoError(t, err)
		assert.Equal(t, Identity{Name: "Dan Pock", Email: "dan@example.com"}, got)
	})

	tests := []struct {
		name string
		cfg  map[string]string
		want []string
	}{
		{"no name", map[string]string{"user.email": "a@b.c"}, []string{"user.name"}},
		{"no email", map[string]string{"user.name": "A"}, []string{"user.email"}},
		{"neither", nil, []string{"user.name", "user.email"}},
	}
	for _, tt := range tests {
		t.Run("missing: "+tt.name, func(t *testing.T) {
			_, err := Workstation(context.Background(), gitConfig(tt.cfg))

			require.Error(t, err)
			for _, key := range tt.want {
				assert.Contains(t, err.Error(), key)
			}
		})
	}

	t.Run("a git failure is returned", func(t *testing.T) {
		boom := errors.New("git is gone")
		_, err := Workstation(context.Background(), func(context.Context, string) (string, error) { return "", boom })

		require.ErrorIs(t, err, boom)
	})
}

func TestResolve(t *testing.T) {
	git := gitConfig(map[string]string{"user.name": "Dan Pock", "user.email": "dan@example.com"})

	t.Run("CI is the bot", func(t *testing.T) {
		got, err := Resolve(context.Background(), Inputs{CI: true, BotUser: "gero-bot[bot]", Git: git, UserID: userID(7)})

		require.NoError(t, err)
		assert.Equal(t, Identity{Name: "gero-bot[bot]", Email: "7+gero-bot[bot]@users.noreply.github.com"}, got)
	})

	t.Run("CI without a bot login fails fast", func(t *testing.T) {
		_, err := Resolve(context.Background(), Inputs{CI: true, Git: git, UserID: userID(7)})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "GERO_BOT_USER")
	})

	t.Run("a workstation is the engineer, and ignores a bot login", func(t *testing.T) {
		got, err := Resolve(context.Background(), Inputs{BotUser: "gero-bot[bot]", Git: git, UserID: userID(7)})

		require.NoError(t, err)
		assert.Equal(t, Identity{Name: "Dan Pock", Email: "dan@example.com"}, got)
	})
}
