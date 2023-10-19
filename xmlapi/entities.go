package xmlapi

//////////////////////////////////////////////////

type Link struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type Author struct {
	Name string `xml:"name"`
	URI  string `xml:"uri"`
}
