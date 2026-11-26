package main

import "encoding/xml"

type nfoEpisode struct {
	XMLName       xml.Name          `xml:"episodedetails"`
	LogicalName   string            `xml:"logicalname,omitempty"`
	Title         string            `xml:"title,omitempty"`
	ShowTitle     string            `xml:"showtitle,omitempty"`
	Season        int               `xml:"season"`
	Episode       int               `xml:"episode"`
	Plot          string            `xml:"plot,omitempty"`
	Aired         string            `xml:"aired,omitempty"`
	Runtime       int               `xml:"runtime,omitempty"`
	Rating        float64           `xml:"rating,omitempty"`
	Votes         int               `xml:"votes,omitempty"`
	UniqueIDs     []nfoUniqueID     `xml:"uniqueid"`
	ImdbID        string            `xml:"imdbid,omitempty"`
	Thumb         string            `xml:"thumb,omitempty"`
	StreamDetails *nfoStreamDetails `xml:"streamdetails,omitempty"`
}

type nfoSeason struct {
	XMLName      xml.Name      `xml:"season"`
	LogicalName  string        `xml:"logicalname,omitempty"`
	Title        string        `xml:"title,omitempty"`
	Plot         string        `xml:"plot,omitempty"`
	Outline      string        `xml:"outline,omitempty"`
	Premiered    string        `xml:"premiered,omitempty"`
	ReleaseDate  string        `xml:"releasedate,omitempty"`
	Year         string        `xml:"year,omitempty"`
	SeasonNumber int           `xml:"seasonnumber"`
	UniqueIDs    []nfoUniqueID `xml:"uniqueid"`
	TmdbID       int           `xml:"tmdbid,omitempty"`
	DoubanID     string        `xml:"doubanid,omitempty"`
	Thumbs       []nfoThumb    `xml:"thumb,omitempty"`
}

type nfoTVShow struct {
	XMLName       xml.Name      `xml:"tvshow"`
	LogicalName   string        `xml:"logicalname,omitempty"`
	Title         string        `xml:"title"`
	OriginalTitle string        `xml:"originaltitle,omitempty"`
	Year          string        `xml:"year,omitempty"`
	Premiered     string        `xml:"premiered,omitempty"`
	Plot          string        `xml:"plot,omitempty"`
	Tagline       string        `xml:"tagline,omitempty"`
	Rating        float64       `xml:"rating,omitempty"`
	Votes         int           `xml:"votes,omitempty"`
	UniqueIDs     []nfoUniqueID `xml:"uniqueid"`
	ImdbID        string        `xml:"imdbid,omitempty"`
	TmdbID        int           `xml:"tmdbid,omitempty"`
	DoubanID      string        `xml:"doubanid,omitempty"`
	Genres        []string      `xml:"genre,omitempty"`
	Studios       []string      `xml:"studio,omitempty"`
	Countries     []string      `xml:"country,omitempty"`
	Thumbs        []nfoThumb    `xml:"thumb,omitempty"`
	Fanart        *nfoFanart    `xml:"fanart,omitempty"`
}
