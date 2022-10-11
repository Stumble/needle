package musicsrepo

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/stumble/needle-clients/mysql/testsuite"
)

const (
	testDB = "test_db"
)

type musicTableCodec struct {
	repo Musics
}

func (m musicTableCodec) Dump() ([]byte, error) {
	return m.repo.Dump(context.Background(), func(m *Music) {
		m.CreatedAt = time.Unix(0, 0)
		m.UpdatedAt = time.Unix(0, 0)
	})
}
func (m musicTableCodec) Load(data []byte) error {
	return m.repo.Load(context.Background(), data)
}

type musicTestSuite struct {
	testsuite.MysqlTestSuite
	repo Musics
}

func TestMusicTestSuite(t *testing.T) {
	suite.Run(t, &musicTestSuite{
		MysqlTestSuite: *testsuite.NewMysqlTestSuite(testDB, []string{
			CreateTableStmt,
		}),
	})
}

func (suite *musicTestSuite) SetupTest() {
	suite.MysqlTestSuite.SetupTest()
	suite.repo = NewMusics(nil, suite.Manager.GetExec())
}

func (suite *musicTestSuite) TestInsertUseGolden() {
	_, err := suite.repo.InsertMusic(context.Background(), &InsertMusicArgs{
		Author:       "Alice",
		Name:         "Bob Land",
		Album:        "Crazy ideas",
		SpotifyID:    999,
		DownloadPath: nil,
		ReleasedAt:   time.Unix(1000, 0),
	})
	suite.Require().NoError(err)
	suite.Golden("musics", musicTableCodec{repo: suite.repo})
}

func (suite *musicTestSuite) TestQueryByID() {
	suite.LoadState("alice_data_set.input.json", musicTableCodec{repo: suite.repo})
	musics, err := suite.repo.ListMusicsLTSpotifyID(context.Background(),
		&ListMusicsLTSpotifyIDArgs{
			SpotifyID: 999,
		})
	suite.Require().NoError(err)
	suite.Equal(2, len(musics))
	str, err := json.Marshal(musics)
	suite.Require().NoError(err)
	suite.Equal(`[{"author":"Alice","name":"Bob Land 2","album":"Crazy ideas","spotify_id":1000,"released_at":"1999-01-01T00:16:40Z","created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"},{"author":"Alice","name":"No more bob","album":"Crazy ideas","spotify_id":1001,"released_at":"2000-01-01T00:16:40Z","created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"}]`, string(str))
}

func (suite *musicTestSuite) TestQuerySearch() {
	suite.LoadState("alice_data_set.input.json", musicTableCodec{repo: suite.repo})
	musics, err := suite.repo.Search(context.Background(), &SearchArgs{Name: "Bob%"})
	suite.Require().NoError(err)
	suite.Equal(2, len(musics))
	str, err := json.Marshal(musics)
	suite.Require().NoError(err)
	suite.Equal(`[{"author":"Alice","name":"Bob Land","album":"Crazy ideas","spotify_id":999,"released_at":"1998-01-01T00:16:40Z","created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"},{"author":"Alice","name":"Bob Land 2","album":"Crazy ideas","spotify_id":1000,"released_at":"1999-01-01T00:16:40Z","created_at":"1970-01-01T00:00:00Z","updated_at":"1970-01-01T00:00:00Z"}]`, string(str))
}
