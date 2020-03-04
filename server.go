package main

import (
	"net/http"
	"github.com/labstack/echo"
	"github.com/labstack/echo/middleware"
	"github.com/buger/jsonparser"
  	"github.com/go-resty/resty"
  	"fmt"
  	"os"
  	"strings"
  	"strconv"
  	"github.com/jinzhu/gorm"
  	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

type County struct {
  gorm.Model
  Name string
  Tax float64
}


func main() {
	// Echo instance
	e := echo.New()

	// Retrieve Data from API
	client := resty.New()
	URL := "https://data.mo.gov/api/views/vpge-tj3s/rows.json?accessType=DOWNLOAD"
	res, err := client.R().
		Get(URL)

	// Error handling
	if err != nil{
		panic(err)
	}
	if res.StatusCode() != 200{
		panic(res.StatusCode())
	}

	// Parse the data
	data , _, _, err := jsonparser.Get(res.Body(), "data")
	if err != nil {
		panic("Fail to parse the data")
	}
	s := string(data)
	bracket := strings.Split(s, "[")

	// Delete old Database
	err = os.Remove("test.db")
    if err != nil {
    	fmt.Println(err)
    }

	// Build SQL Database
	db, err := gorm.Open("sqlite3", "test.db")
    if err != nil {
      panic("failed to connect database")
    }
    defer db.Close()

    // Migrate the schema
  	db.AutoMigrate(&County{})

  	// Create the Database
  	index := -1
  	var key string
  	var value float64
  	for i := 0; i < len(bracket); i++ {
  		comma := strings.Split(bracket[i], ",")
  		if len(comma) < 10 {
  			continue
  		}
  		index += 1
  		if index%2 == 0 {
			if (len(comma) > 10 && len(comma[8]) > 7) {
				key = comma[8][2:len(comma[8])-1]
				value, _ = strconv.ParseFloat(comma[9][2:len(comma[9])-2], 64)
			}
  		}
  		if index%2 == 1 {
  			if (!strings.Contains(bracket[i], "city")){
  				continue
  			}
  			key += " - " + comma[1][13:len(comma[1])-2]
  			db.Create(&County{Name: key, Tax: value})
  		}
	}


	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	// GET API
	e.GET("/", home)
	e.GET("/data/:type", getData) // ex: http://localhost:8080/data/json?name=Dallas County

	// POST API
	e.POST("/data", createData)   // ex: http://localhost:8080/data/name=Dallas County&value=0.0825
	// PUT API
	e.PUT("/data", updateData)    // ex: http://localhost:8080/data/name=Dallas County&value=0.0825
	// DELETE
	e.DELETE("/data", deleteData) // ex: http://localhost:8080/data/name=Dallas County


	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}

// Delete the data from the database
func deleteData(c echo.Context) error {

	// Handle request parameter
	name := c.QueryParam("name")

	// Connect to the database
	db, err := gorm.Open("sqlite3", "test.db")
    if err != nil {
      panic("failed to connect database")
    }
    defer db.Close()

    // Check if the data already exsists
    var product County
    temp := db.First(&product, "name = ?", name).Value
    temp1 := temp.(*County)
    if temp1.Tax == 0 {
    	return c.String(http.StatusBadRequest, fmt.Sprintf("%s doesn't exist. Failed to delete.", name))
    }

    // Delete the data
    db.Delete(&product)

    return c.String(http.StatusOK, fmt.Sprintf("County Name: %s has been deleted. ", name))


}

func updateData(c echo.Context) error {

	// Handle request parameter
	name := c.QueryParam("name")
	value := c.QueryParam("value")

	// Connect to the database
	db, err := gorm.Open("sqlite3", "test.db")
    if err != nil {
      panic("failed to connect database")
    }
    defer db.Close()

    // Check if the data already exsists
    var product County
    temp := db.First(&product, "name = ?", name).Value
    temp1 := temp.(*County)
    if temp1.Tax == 0 {
    	return c.String(http.StatusBadRequest, fmt.Sprintf("%s doesn't exist. Failed to update.", name))
    }

    // Update the data
    flo, _ := strconv.ParseFloat(value, 64)

    db.Delete(&product)
    db.Create(&County{Name: name, Tax: flo})

    return c.String(http.StatusOK, fmt.Sprintf("County Name: %s Tax Rate: %.4f updated.", name, flo))
}


// Add a new data to the database - POST API
func createData(c echo.Context) error {

	// Handle request parameter
	name := c.QueryParam("name")
	value := c.QueryParam("value")

	// Connect to the database
	db, err := gorm.Open("sqlite3", "test.db")
    if err != nil {
      panic("failed to connect database")
    }
    defer db.Close()

    // Check if the data already exsists
    var product County
    temp := db.First(&product, "name = ?", name).Value
    temp1 := temp.(*County)
    if temp1.Tax != 0 {
    	return c.String(http.StatusBadRequest, fmt.Sprintf("%s already exists. Failed to create.", name))
    }

    // Add the data
    flo, _ := strconv.ParseFloat(value, 64)
    db.Create(&County{Name: name, Tax: flo})

    return c.String(http.StatusOK, fmt.Sprintf("County Name: %s Tax Rate: %.4f created.", name, flo))
}

// Retrieve data from the database - GET API
func getData(c echo.Context) error {
	
	// Handle request parameter
	name := c.QueryParam("name")
	datatype := c.Param("type")

	// Connect to the Database
	db, err := gorm.Open("sqlite3", "test.db")
    if err != nil {
      panic("failed to connect database")
    }
    defer db.Close()

    // Retrieve the data from database
	var product County
    temp := db.First(&product, "name = ?", name).Value
    temp1 := temp.(*County)

    // Handle string return type
    if datatype == "string" {
    	if temp1.Tax == 0 {
			return c.String(http.StatusBadRequest, fmt.Sprintf("There's no %s found in the database.", name))
		}
		return c.String(http.StatusOK, fmt.Sprintf("County Name: %s\nTax Rate: %.2f %s", name, temp1.Tax*100, "%"))
    }

    // Handle json return type
    if datatype == "json" {
    	if name == "all" {
	    	var county County
		    rows, err := db.Model(&County{}).Rows()
		    defer rows.Close()
		    if err != nil {
		        panic(err)
		    }
		    respond := make(map[string]string)
		    for rows.Next() {
	        	db.ScanRows(rows, &county)
	        	temp := strconv.FormatFloat(county.Tax, 'f', 2, 64)
	        	respond[county.Name] = temp
	    	}
	    	return c.JSON(http.StatusOK, respond)
    	}
    	if temp1.Tax == 0.0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"County Name": "Not Found", "Tax Rate": "Not Found"})
		}
    	return c.JSON(http.StatusOK, map[string]string{"County Name": name, "Tax Rate": strconv.FormatFloat(temp1.Tax*100, 'f', 2, 64)})
    }
    // Handle invalid datatype
    return c.String(http.StatusBadRequest, fmt.Sprintf("Please specify return datatype. string or json"))
}

// Handle home page
func home(c echo.Context) error {
	// Connect to the database
	db, err := gorm.Open("sqlite3", "test.db")
    if err != nil {
      panic("failed to connect database")
    }
    defer db.Close()

    // Retrieve all the data from the database
    var county County
    rows, err := db.Model(&County{}).Rows()
    defer rows.Close()
    if err != nil {
        panic(err)
    }

    respond := "<h> County Tax Rate <h>"
    for rows.Next() {
        db.ScanRows(rows, &county)
        temp := strconv.FormatFloat(county.Tax*100, 'f', 2, 64)
        respond += "<li>" + county.Name +" : " + temp + "%" + "</li>"
    }

	return c.HTML(http.StatusOK, respond)
}
