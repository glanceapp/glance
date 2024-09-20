package feed

import (
    "encoding/xml"
    "io"
    "net/http"
    "strings"
)

type BGGFeedResponseXML struct {
    XMLName xml.Name    `xml:"items"`
    Items   []struct {
        XMLName xml.Name    `xml:"item"`
        ID string   `xml:"id,attr"`
        Thumbnail   struct {
            Value   string  `xml:"value,attr"`
        }   `xml:"thumbnail"`
        Name   struct {
            Value   string  `xml:"value,attr"`
        }   `xml:"name"`
        YearPublished   struct {
            Value   string  `xml:"value,attr"`
        }   `xml:"yearpublished"`
        Rank    string  `xml:"rank,attr"`
    }   `xml:"item"`
}

func FetchBGGHotnessList() (BggBoardGames, error){
    resp, err := http.Get("https://boardgamegeek.com/xmlapi2/hot?boardgame")
    if err != nil {
        return BggBoardGames{}, err
    }

    defer resp.Body.Close()

    data, err := io.ReadAll(resp.Body)
    if err != nil {
        return BggBoardGames{}, err
    }

    var hotnessFeed BGGFeedResponseXML
    err = xml.Unmarshal(data, &hotnessFeed)
    if err != nil {
        return BggBoardGames{}, err
    }

    bggGames := getItemsFromBGGFeedTask(hotnessFeed)
    
    return bggGames, nil
}

func getItemsFromBGGFeedTask(response BGGFeedResponseXML) (BggBoardGames) {
    games := make(BggBoardGames, 0, len(response.Items))

    for _, item := range response.Items {
        splitUrl :=  strings.Split(item.Thumbnail.Value, "filters:strip_icc()")
        thumbUrl := BggThumbnailUrl { splitUrl[0], splitUrl[1]}
        bggBoardGame := BggBoardGame {
            ID:             item.ID,
            ThumbnailUrl:   thumbUrl, 
            Name:           item.Name.Value,
            YearPublished:  item.YearPublished.Value,
            Rank:           item.Rank,
            BGGGameLink:    "https://boardgamegeek.com/boardgame/" + item.ID,
        }

        games = append(games, bggBoardGame)
    }

    return games
}
