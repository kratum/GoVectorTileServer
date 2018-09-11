package main

import (
	"database/sql"
	"fmt"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
)

//Edit cs with your db connection data
var cs = ""
var db *sql.DB

func init() {
	db, err := sql.Open("postgres", cs)
	check(err)

	err = db.Ping()
	check(err)

	defer db.Close()
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/mvt/{z}/{y}/{x}", mvtHandler).Methods("GET", "OPTIONS")
	r.Handle("/favicon.ico", http.NotFoundHandler())

	//Run the Server and Log Errors
	log.Fatal(http.ListenAndServe(":8080", r))
}

func mvtHandler(w http.ResponseWriter, req *http.Request) {
	db, err := sql.Open("postgres", cs)
	check(err)

	defer db.Close()

	w.Header().Set("Content-Type", "application/x-protobuf")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	params := mux.Vars(req)

	x, err := strconv.Atoi(params["x"])
	y, err := strconv.Atoi(params["y"])
	z, err := strconv.Atoi(params["z"])

	bbox := GetBbox(x, y, z)
	tilefolder := fmt.Sprintf("./%s/%d/%d", "cache", z, x)
	tilepath := fmt.Sprintf("%s/%d.pbf", tilefolder, y)

	fmt.Println(tilefolder, tilepath)

	if _, err := os.Stat(tilepath); os.IsNotExist(err) {
		fmt.Println("Generating new Tile")

		stmt, err := db.Prepare(`SELECT ST_AsMVT(q, 'merged_clean_vol_poly', 4096, 'geom')
	            FROM (
	                SELECT
	                    vol,
	                    id,
	                    ST_AsMVTGeom(
	                        geom,
	                        TileBBox($1, $2, $3, 3857),
	                        4096,
	                        0,
	                        false
	                    ) geom
	                FROM merged_clean_vol_poly
	                WHERE ST_Intersects(geom, (SELECT ST_Transform(ST_MakeEnvelope($4,$5,$6,$7, 4326), 3857)))
	            ) q`)
		check(err)

		rows, err := stmt.Query(z, x, y, bbox[0], bbox[1], bbox[2], bbox[3])
		check(err)

		for rows.Next() {
			var st_asmvt string
			err = rows.Scan(&st_asmvt)
			check(err)
			w.Write([]byte(st_asmvt))

			if _, err := os.Stat(tilefolder); os.IsNotExist(err) {
				os.MkdirAll(tilefolder, 1)
			}
			err = ioutil.WriteFile(tilepath, []byte(st_asmvt), 1)
			check(err)
		}

		//jsonFile, err := os.Open("./data.json")
	} else {
		RawTile, err := os.Open(tilepath)
		check(err)
		tile, _ := ioutil.ReadAll(RawTile)
		w.Write(tile)
	}

}

//Errorhandling
func check(e error) {
	if e != nil {
		panic(e)
	}
}

// TileUl is a remake from Giovanni Allegris function in
// https://medium.com/tantotanto/vector-tiles-postgis-and-openlayers-258a3b0ce4b6
func TileUl(x int, y int, z int) (float64, float64) {
	n := math.Pow(2, float64(z))
	lon_deg := float64(x)/n*360.0 - 180.0
	lat_rad := math.Atan(math.Sinh(math.Pi * (1 - 2*float64(y)/n)))
	lat_deg := lat_rad * (180 / math.Pi)
	return lon_deg, lat_deg
}

// GetBbox is a remake from Mapbox SphericalMercator.bbox
// https://github.com/mapbox/sphericalmercator/blob/master/sphericalmercator.js
func GetBbox(x int, y int, z int) []float64 {
	xmin, ymin := TileUl(x, y, z)
	xmax, ymax := TileUl(x+1, y+1, z)

	bbox := []float64{xmin, ymin, xmax, ymax}
	return bbox
}
