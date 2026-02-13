package models

import "encoding/xml"

// HolidayType represents the type of a holiday/special day
type HolidayType string

const (
	HolidayTypeHoliday      HolidayType = "holiday"
	HolidayTypeShortened    HolidayType = "shortened"
	HolidayTypeMovedWeekend HolidayType = "moved_weekend"
)

// Holiday represents a single holiday entry
type Holiday struct {
	Date string      `json:"date"` // YYYY-MM-DD
	Name string      `json:"name"`
	Type HolidayType `json:"type"`
}

// HolidaysResponse is the API response for the holidays endpoint
type HolidaysResponse struct {
	Year     int       `json:"year"`
	Holidays []Holiday `json:"holidays"`
}

// XMLCalendar represents the parsed XML structure from xmlcalendar.ru
type XMLCalendar struct {
	XMLName  xml.Name       `xml:"calendar"`
	Year     string         `xml:"year,attr"`
	Lang     string         `xml:"lang,attr"`
	Country  string         `xml:"country,attr"`
	Holidays XMLHolidayList `xml:"holidays"`
	Days     XMLDayList     `xml:"days"`
}

// XMLHolidayList is a list of holiday definitions
type XMLHolidayList struct {
	Items []XMLHolidayItem `xml:"holiday"`
}

// XMLHolidayItem represents a holiday definition with id and title
type XMLHolidayItem struct {
	ID    string `xml:"id,attr"`
	Title string `xml:"title,attr"`
}

// XMLDayList is a list of special days
type XMLDayList struct {
	Items []XMLDayItem `xml:"day"`
}

// XMLDayItem represents a single special day entry
// d: date in MM.DD format
// t: type (1=holiday, 2=shortened, 3=working weekend)
// h: holiday id reference (optional)
// f: moved from date (optional, for t=3)
type XMLDayItem struct {
	Date      string `xml:"d,attr"`
	Type      string `xml:"t,attr"`
	HolidayID string `xml:"h,attr"`
	From      string `xml:"f,attr"`
}
