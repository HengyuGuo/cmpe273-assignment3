package main

import (  
    "bytes"
    "fmt"
    "net/http"
    "os"
    "encoding/json"
    "assignment2/controllers"
    "assignment2/models"
    "assignment2/permutation"
    "io/ioutil"
    "math/rand"
    "strconv"
    // Third party packages
    "gopkg.in/mgo.v2"
    "gopkg.in/mgo.v2/bson"
    "github.com/julienschmidt/httprouter"
)

var id int
var hmap map[int]DataStorage

type UberAPIResponse struct {
    Prices []struct {
        CurrencyCode    string  `json:"currency_code"`
        DisplayName     string  `json:"display_name"`
        Distance        float64 `json:"distance"`
        Duration        int     `json:"duration"`
        Estimate        string  `json:"estimate"`
        HighEstimate    int     `json:"high_estimate"`
        LowEstimate     int     `json:"low_estimate"`
        ProductID       string  `json:"product_id"`
        SurgeMultiplier int     `json:"surge_multiplier"`
    } `json:"prices"`
}

type TripRequest struct {
    Starting_from_location_id bson.ObjectId
    Location_ids              []bson.ObjectId
}

type TripResponse struct {
    Id                        int
    Status                    string
    Starting_from_location_id bson.ObjectId
    Best_route_location_id    []bson.ObjectId
    Total_uber_costs          int
    Total_uber_duration       int
    Total_distance            float64
}

type DataStorage struct {
    Id                        int
    // Product_id                string
    Index                     int
    Status                    string
    Starting_from_location_id bson.ObjectId
    Best_route_location_id    []bson.ObjectId
    Total_uber_costs          int
    Total_uber_duration       int
    Total_distance            float64
}

type UberRequestResponse struct {
    RequestID       string  `json:"request_id"`
    Status          string  `json:"status"`
    Vehicle         string  `json:"vehicle"`
    Driver          string  `json:"driver"`
    Location        string  `json:"location"`
    ETA             int     `json:"eta"`
    SurgeMultiplier float64 `json:"surge_multiplier"`
}

type CarResponse struct {
    Id                           int
    Status                       string
    Starting_from_location_id    bson.ObjectId
    Next_destination_location_id bson.ObjectId
    Best_route_location_id       []bson.ObjectId
    Total_uber_costs             int
    Total_uber_duration          int
    Total_distance               float64
    Uber_wait_time_eta           int
}

type UserRequest struct {
    Product_id      string  `json:"product_id"`
    Start_latitude  float64 `json:"start_latitude"`
    Start_longitude float64 `json:"start_longitude"`
    End_latitude    float64 `json:"end_latitude"`
    End_longitude   float64 `json:"end_longitude"`
}

type UberResponse struct {
    Driver          interface{} `json:"driver"`
    Eta             int         `json:"eta"`
    Location        interface{} `json:"location"`
    RequestID       string      `json:"request_id"`
    Status          string      `json:"status"`
    SurgeMultiplier float64     `json:"surge_multiplier"`
    Vehicle         interface{} `json:"vehicle"`
}

func getSession() *mgo.Session {  
    s, err := mgo.Dial("mongodb://admin:123@ds041404.mongolab.com:41404/cmpe273")
    if err != nil {
        fmt.Println("Can't connect to mongo, go error %v\n", err)
        os.Exit(1)
    }
    return s
}

func main() {
    id = 0
    hmap = make(map[int]DataStorage)

    mux := httprouter.New()
    uc := controllers.NewUserController(getSession())
    mux.GET("/locations/:id", uc.GetLocations)
    mux.POST("/locations/", uc.CreateLocations)
    mux.DELETE("/locations/:id", uc.RemoveLocations)
    mux.PUT("/locations/:id", uc.UpdateLocations)
    mux.GET("/trips/:trip_id", GetTrip)
    mux.POST("/trips/", CreateTrip)
    mux.PUT("/trips/:trip_id/request", CarRequest)
    server := http.Server{
            Addr:        "0.0.0.0:8080",
            Handler: mux,
    }
    server.ListenAndServe()
}

func CreateTrip(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
    var tripReq TripRequest
    json.NewDecoder(req.Body).Decode(&tripReq)

    dataStorage := DataStorage{}
    tripRes := TripResponse{}
    tripRes.Id = getID()
    tripRes.Status = "planning"
    tripRes.Starting_from_location_id = tripReq.Starting_from_location_id
    tripRes.Best_route_location_id = tripReq.Location_ids
    getBestRoute(&tripRes, &dataStorage, tripReq.Starting_from_location_id, tripReq.Location_ids)

    fmt.Println(dataStorage)

    hmap[tripRes.Id] = dataStorage

    trip, _ := json.Marshal(tripRes)
    rw.Header().Set("Content-Type", "application/json")
    rw.WriteHeader(201)
    fmt.Fprintf(rw, "%s", trip)
}

func GetTrip(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
    tId := p.ByName("trip_id")
    checkID, _ := strconv.Atoi(tId)
    var dataStorage DataStorage
    findTarget := false

    for key, value := range hmap {
        if key == checkID {
            dataStorage = value
            findTarget = true
        }
    }

    if findTarget == false {
        w.WriteHeader(404)
        return
    }

    tripRes := TripResponse{}
    tripRes.Id = dataStorage.Id
    tripRes.Status = dataStorage.Status
    tripRes.Starting_from_location_id = dataStorage.Starting_from_location_id
    tripRes.Best_route_location_id = dataStorage.Best_route_location_id
    tripRes.Total_uber_costs = dataStorage.Total_uber_costs
    tripRes.Total_distance = dataStorage.Total_distance
    tripRes.Total_uber_duration = dataStorage.Total_uber_duration

    trip, _ := json.Marshal(tripRes)
    w.WriteHeader(200)
    fmt.Fprintf(w, "%s", trip)
}

func CarRequest(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
    tId := p.ByName("trip_id")
    checkID, _ := strconv.Atoi(tId)
    var dataStorage DataStorage
    findTarget := false

    for key, value := range hmap {
        if key == checkID {
            dataStorage = value
            findTarget = true
        }
    }

    if findTarget == false {
        w.WriteHeader(404)
        return
    }

    var startLat float64
    var startLng float64
    var endLat float64
    var endLng float64
    carRes := CarResponse{}
    response := models.Location{}

    if dataStorage.Index == 0 {
        if err := getSession().DB("cmpe273").C("assignment2").FindId(dataStorage.Starting_from_location_id).One(&response); err != nil {
            return
        }
        startLat = response.Coordinate.Lat
        startLng = response.Coordinate.Lng

        if err := getSession().DB("cmpe273").C("assignment2").FindId(dataStorage.Best_route_location_id[0]).One(&response); err != nil {
            return
        }
        endLat = response.Coordinate.Lat
        endLng = response.Coordinate.Lng
        uberAPI(&carRes, dataStorage, startLat, startLng, endLat, endLng)
        carRes.Status = "requesting"
        carRes.Starting_from_location_id = dataStorage.Starting_from_location_id
        carRes.Next_destination_location_id = dataStorage.Best_route_location_id[0]
    } else if dataStorage.Index == len(dataStorage.Best_route_location_id) {
        if err := getSession().DB("cmpe273").C("assignment2").FindId(dataStorage.Best_route_location_id[len(dataStorage.Best_route_location_id)-1]).One(&response); err != nil {
            return
        }
        startLat = response.Coordinate.Lat
        startLng = response.Coordinate.Lng

        if err := getSession().DB("cmpe273").C("assignment2").FindId(dataStorage.Starting_from_location_id).One(&response); err != nil {
            return
        }
        endLat = response.Coordinate.Lat
        endLng = response.Coordinate.Lng
        uberAPI(&carRes, dataStorage, startLat, startLng, endLat, endLng)
        carRes.Status = "requesting"
        carRes.Starting_from_location_id = dataStorage.Best_route_location_id[len(dataStorage.Best_route_location_id)-1]
        carRes.Next_destination_location_id = dataStorage.Starting_from_location_id
    } else if dataStorage.Index > len(dataStorage.Best_route_location_id) {
        carRes.Status = "finished"
        carRes.Starting_from_location_id = dataStorage.Starting_from_location_id
        carRes.Next_destination_location_id = dataStorage.Starting_from_location_id
    } else {
        if err := getSession().DB("cmpe273").C("assignment2").FindId(dataStorage.Best_route_location_id[dataStorage.Index-1]).One(&response); err != nil {
            return
        }
        startLat = response.Coordinate.Lat
        startLng = response.Coordinate.Lng

        if err := getSession().DB("cmpe273").C("assignment2").FindId(dataStorage.Best_route_location_id[dataStorage.Index]).One(&response); err != nil {
            return
        }
        endLat = response.Coordinate.Lat
        endLng = response.Coordinate.Lng
        uberAPI(&carRes, dataStorage, startLat, startLng, endLat, endLng)
        carRes.Status = "requesting"
        carRes.Starting_from_location_id = dataStorage.Best_route_location_id[dataStorage.Index-1]
        carRes.Next_destination_location_id = dataStorage.Best_route_location_id[dataStorage.Index]
    }

    carRes.Id = dataStorage.Id
    carRes.Best_route_location_id = dataStorage.Best_route_location_id
    carRes.Total_uber_costs = dataStorage.Total_uber_costs
    carRes.Total_uber_duration = dataStorage.Total_uber_duration
    carRes.Total_distance = dataStorage.Total_distance

    fmt.Println(dataStorage.Index)
    dataStorage.Index = dataStorage.Index + 1
    hmap[dataStorage.Id] = dataStorage

    fmt.Println(dataStorage.Index)

    trip, _ := json.Marshal(carRes)
    w.WriteHeader(200)
    fmt.Fprintf(w, "%s", trip)
}

func getBestRoute(tripRes *TripResponse, dataStorage *DataStorage, originId bson.ObjectId, targetId []bson.ObjectId) {
    pmtTarget, err := permutation.NewPerm(targetId, nil)
    if err != nil {
        fmt.Println(err)
        return
    }
    res := make([][]bson.ObjectId, 0, 0)
    routePrice := make([]int, 0, 0)
    routeDuration := make([]int, 0, 0)
    routeDistance := make([]float64, 0, 0)
    curPrice := 0
    curDuration := 0
    curDistance := 0.0
    for result,err := pmtTarget.Next(); err == nil; result,err = pmtTarget.Next() {
        //----------------------get Price---------------------------------
        for i := 0; i <= len(result.([]bson.ObjectId)); i++ {
            var startLat float64
            var startLng float64
            var endLat float64
            var endLng float64
            minPrice := 0
            minDuration := 0
            minDistance := 0.0
            response := models.Location{}
            if i == 0 {
                if err := getSession().DB("cmpe273").C("assignment2").FindId(originId).One(&response); err != nil {
                    return
                }
                startLat = response.Coordinate.Lat
                startLng = response.Coordinate.Lng

                if err := getSession().DB("cmpe273").C("assignment2").FindId((result.([]bson.ObjectId))[i]).One(&response); err != nil {
                    return
                }
                endLat = response.Coordinate.Lat
                endLng = response.Coordinate.Lng
            }else if i == len(result.([]bson.ObjectId)) {
                if err := getSession().DB("cmpe273").C("assignment2").FindId((result.([]bson.ObjectId))[i - 1]).One(&response); err != nil {
                    return
                }
                startLat = response.Coordinate.Lat
                startLng = response.Coordinate.Lng

                if err := getSession().DB("cmpe273").C("assignment2").FindId(originId).One(&response); err != nil {
                    return
                }
                endLat = response.Coordinate.Lat
                endLng = response.Coordinate.Lng
            }else {
                if err := getSession().DB("cmpe273").C("assignment2").FindId((result.([]bson.ObjectId))[i - 1]).One(&response); err != nil {
                    return
                }
                startLat = response.Coordinate.Lat
                startLng = response.Coordinate.Lng

                if err := getSession().DB("cmpe273").C("assignment2").FindId((result.([]bson.ObjectId))[i]).One(&response); err != nil {
                    return
                }
                endLat = response.Coordinate.Lat
                endLng = response.Coordinate.Lng
            }

            urlLeft := "https://api.uber.com/v1/estimates/price?"
            urlRight := "start_latitude=" + strconv.FormatFloat(startLat, 'f', -1, 64) + "&start_longitude=" + strconv.FormatFloat(startLng, 'f', -1, 64) + "&end_latitude=" + strconv.FormatFloat(endLat, 'f', -1, 64) + "&end_longitude=" + strconv.FormatFloat(endLng, 'f', -1, 64) + "&server_token=zoo7SgqJUZCaVNsOAKgiRgvJsv3l8cavZiB6sD07"
            urlFormat := urlLeft + urlRight

            getPrices, err := http.Get(urlFormat)
            if err != nil {
                fmt.Println("Get Prices Error", err)
                panic(err)
            }
            var data UberAPIResponse
            json.NewDecoder(getPrices.Body).Decode(&data)
            minPrice = data.Prices[0].LowEstimate
            minDuration = data.Prices[0].Duration
            minDistance = data.Prices[0].Distance
            for i := 0; i < len(data.Prices); i++ {
                if minPrice > data.Prices[i].LowEstimate && data.Prices[i].LowEstimate > 0 {
                    minPrice = data.Prices[i].LowEstimate
                    minDuration = data.Prices[i].Duration
                    minDistance = data.Prices[i].Distance
                }
            }
            curPrice = curPrice + minPrice
            curDuration = curDuration + minDuration
            curDistance = curDistance + minDistance
        }
        //----------------------get Price---------------------------------
        routePrice = AppendInt(routePrice, curPrice)
        routeDuration = AppendInt(routeDuration, curDuration)
        routeDistance = AppendFloat(routeDistance, curDistance)
        fmt.Println(curPrice)
        fmt.Println(curDuration)
        fmt.Println(curDistance)
        curPrice = 0
        curDuration = 0
        curDistance = 0.0
        res = AppendBsonId(res, result.([]bson.ObjectId))
        fmt.Println(pmtTarget.Index(), result.([]bson.ObjectId))
    }
    index := 0
    curPrice = 1000
    for i := 0; i < len(routePrice); i++ {
        if curPrice > routePrice[i] {
            curPrice = routePrice[i]
            index = i
        }
    }
    fmt.Println("best route is => ")
    fmt.Println(res[index])
    fmt.Println(routePrice[index])
    fmt.Println(routeDuration[index])
    fmt.Println(routeDistance[index])
    tripRes.Best_route_location_id = res[index]
    tripRes.Total_uber_costs = routePrice[index]
    tripRes.Total_uber_duration = routeDuration[index]
    tripRes.Total_distance = routeDistance[index]

    dataStorage.Id = tripRes.Id
    dataStorage.Index = 0
    dataStorage.Status = tripRes.Status
    dataStorage.Starting_from_location_id = tripRes.Starting_from_location_id
    dataStorage.Best_route_location_id = res[index]
    dataStorage.Total_distance = tripRes.Total_distance
    dataStorage.Total_uber_costs = tripRes.Total_uber_costs
    dataStorage.Total_uber_duration = tripRes.Total_uber_duration
}

func AppendBsonId(slice [][]bson.ObjectId, data ...[]bson.ObjectId) [][]bson.ObjectId {
    m := len(slice)
    n := m + 1
    if n > cap(slice) { 
        newSlice := make([][]bson.ObjectId, (n+1)*2)
        copy(newSlice, slice)
        slice = newSlice
    }
    slice = slice[0:n]
    copy(slice[m:n], data)
    return slice
}

func AppendInt(slice []int, data ...int) []int {
    m := len(slice)
    n := m + 1
    if n > cap(slice) { // if necessary, reallocate
        // allocate double what's needed, for future growth.
        newSlice := make([]int, (n+1)*2)
        copy(newSlice, slice)
        slice = newSlice
    }
    slice = slice[0:n]
    copy(slice[m:n], data)
    return slice
}

func AppendFloat(slice []float64, data ...float64) []float64 {
    m := len(slice)
    n := m + 1
    if n > cap(slice) { 
        newSlice := make([]float64, (n+1)*2)
        copy(newSlice, slice)
        slice = newSlice
    }
    slice = slice[0:n]
    copy(slice[m:n], data)
    return slice
}

func uberAPI(carRes *CarResponse, dataStorage DataStorage, startLat float64, startLng float64, endLat float64, endLng float64) {
    minPrice := 0
    serverToken := "*************************************"
    urlLeft := "https://api.uber.com/v1/estimates/price?"
    urlRight := "start_latitude=" + strconv.FormatFloat(startLat, 'f', -1, 64) + "&start_longitude=" + strconv.FormatFloat(startLng, 'f', -1, 64) + "&end_latitude=" + strconv.FormatFloat(endLat, 'f', -1, 64) + "&end_longitude=" + strconv.FormatFloat(endLng, 'f', -1, 64) + "&server_token=" + serverToken
    urlFormat := urlLeft + urlRight
    var userrequest UserRequest

    getPrices, err := http.Get(urlFormat)
    if err != nil {
        fmt.Println("Get Prices Error", err)
        panic(err)
    }

    var data UberAPIResponse
    index := 0

    json.NewDecoder(getPrices.Body).Decode(&data)

    minPrice = data.Prices[0].LowEstimate
    for i := 0; i < len(data.Prices); i++ {
        if minPrice > data.Prices[i].LowEstimate {
            minPrice = data.Prices[i].LowEstimate
            index = i
        }
        userrequest.Product_id = data.Prices[index].ProductID
    }

    urlPath := "https://sandbox-api.uber.com/v1/requests"
    userrequest.Start_latitude = startLat
    userrequest.Start_longitude = startLng
    userrequest.End_latitude = endLat
    userrequest.End_longitude = endLng
    accessToken := "****************************"

    requestbody, _ := json.Marshal(userrequest)
    client := &http.Client{}
    req, err := http.NewRequest("POST", urlPath, bytes.NewBuffer(requestbody))
    if err != nil {
        fmt.Println(err)
        return
    }
    req.Header.Add("Content-Type", "application/json")
    req.Header.Add("Authorization", "Bearer "+accessToken)
    res, err := client.Do(req)
    if err != nil {
        fmt.Println("QueryInfo: http.Get", err)
        return
    }
    defer res.Body.Close()

    body, err := ioutil.ReadAll(res.Body)
    uberRes := UberResponse{}
    json.Unmarshal(body, &uberRes)

    fmt.Println(uberRes)

    carRes.Uber_wait_time_eta = uberRes.Eta
}

func getID() int {
    if id == 0 {
        for id == 0 {
            id = rand.Intn(10000)
        }
    } else {
        id = id + 1
    }
    return id
}