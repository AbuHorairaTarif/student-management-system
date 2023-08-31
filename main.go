package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"
	"sync"
)

type Student struct {
	Name           string
	ID             int
	CGPA           float64
	CareerInterest string
	ImagePath      string
}

var (
    studentData    = make(map[int]Student)
    studentID      = 1
    studentIDLock  sync.Mutex
    studentAdded   bool
    studentDeleted bool // New flag to track if a student is deleted successfully
)


func main() {

	if err := loadStudentDataFromJSON(); err != nil {
        fmt.Printf("Error loading student data: %v\n", err)
    }

	http.HandleFunc("/", ViewStudents)
	http.HandleFunc("/all_students", ViewAllStudents)
	http.HandleFunc("/add", AddStudent)
	http.HandleFunc("/display", DisplayStudent)
	http.HandleFunc("/delete", DeleteStudent)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	http.ListenAndServe(":8081", nil)
}

func ViewStudents(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.ParseFiles("templates/index.html"))

    // Pass the StudentAdded and StudentDeleted flags to the template
    data := struct {
        Students       []Student
        StudentAdded   bool
        StudentDeleted bool
    }{
        Students:       getStudents(),
        StudentAdded:   studentAdded,
        StudentDeleted: studentDeleted,
    }

    // Reset both flags to false
    studentAdded = false
    studentDeleted = false

    tmpl.Execute(w, data)
}


func AddStudent(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Create a channel to signal completion
    done := make(chan bool)

    go func() {
        // Parse form data
        err := r.ParseMultipartForm(10 << 20) // 10 MB limit
        if err != nil {
            http.Error(w, "Error parsing form data", http.StatusBadRequest)
            done <- false
            return
        }

        name := r.FormValue("name")
        cgpaStr := r.FormValue("cgpa")

        // Validate CGPA
        cgpa, err := strconv.ParseFloat(cgpaStr, 64)
        if err != nil || cgpa < 2.00 || cgpa > 4.00 {
            http.Error(w, "Invalid CGPA. CGPA must be between 2.00 and 4.00.", http.StatusBadRequest)
            done <- false
            return
        }

        careerInterest := r.FormValue("career_interest")

        // Handle image upload
        file, _, err := r.FormFile("image")
        if err != nil {
            http.Error(w, "Error uploading image", http.StatusInternalServerError)
            done <- false
            return
        }
        defer file.Close()

        // Generate a new unique student ID based on the number of existing students
        id := len(studentData) + 1

        // Save the image with a unique filename
        imagePath := fmt.Sprintf("uploads/%d.jpg", id)

        outFile, err := os.Create(imagePath)
        if err != nil {
            http.Error(w, "Error saving image", http.StatusInternalServerError)
            done <- false
            return
        }
        defer outFile.Close()
        io.Copy(outFile, file)

        student := Student{
            Name:           name,
            ID:             id,
            CGPA:           cgpa,
            CareerInterest: careerInterest,
            ImagePath:      imagePath,
        }

        studentData[id] = student

        // After successfully adding the student, set the flag
        studentAdded = true

        // Signal completion
        done <- true
    }()

    // Wait for the Goroutine to finish and check if the operation was successful
    if success := <-done; success {
        // Save student data as JSON in another Goroutine
        go func() {
            saveStudentDataAsJSON()
        }()

        // Redirect back to the main page
        http.Redirect(w, r, "/", http.StatusSeeOther)
    } else {
        // Handle errors and display an error message
        // You can customize this part based on your error handling needs
        fmt.Println("Error processing form submission.")
    }
}



func saveStudentDataAsJSON() {

	
    // Create a channel to signal completion
    done := make(chan bool)

    go func() {
        file, err := os.Create("studentData.json")
        if err != nil {
            fmt.Println("Error creating JSON file:", err)
            done <- false
            return
        }
        defer file.Close()

        data, err := json.MarshalIndent(studentData, "", "    ")
        if err != nil {
            fmt.Println("Error encoding JSON:", err)
            done <- false
            return
        }

        _, err = file.Write(data)
        if err != nil {
            fmt.Println("Error writing JSON data:", err)
            done <- false
            return
        }

        // Signal completion
        done <- true
    }()

    // Wait for the Goroutine to finish and check if the operation was successful
    if success := <-done; success {
        fmt.Println("Student data saved as JSON successfully.")
    } else {
        fmt.Println("Error saving student data as JSON.")
    }
}

func loadStudentDataFromJSON() error {
    file, err := os.Open("studentData.json")
    if err != nil {
        return err
    }
    defer file.Close()

    decoder := json.NewDecoder(file)
    if err := decoder.Decode(&studentData); err != nil {
        return err
    }

    return nil
}



func DisplayStudent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id, err := strconv.Atoi(r.FormValue("display_id"))
	if err != nil {
		http.Error(w, "Invalid Student ID", http.StatusBadRequest)
		return
	}

	student, ok := studentData[id]
	if !ok {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	tmpl := template.Must(template.ParseFiles("templates/student_details.html"))

	tmpl.Execute(w, student)
}

func DeleteStudent(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    id, err := strconv.Atoi(r.FormValue("delete_id"))
    if err != nil {
        http.Error(w, "Invalid Student ID", http.StatusBadRequest)
        return
    }

    _, ok := studentData[id]
    if !ok {
        http.Error(w, "Student not found", http.StatusNotFound)
        return
    }

    // Delete the student
    delete(studentData, id)

    // Set the studentDeleted flag to true
    studentDeleted = true

    // Save student data as JSON after deletion
    saveStudentDataAsJSON()

    // Redirect back to the main page
    http.Redirect(w, r, "/", http.StatusSeeOther)
}




func ViewAllStudents(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.ParseFiles("templates/all_students.html"))

	var students []Student
	for _, student := range studentData {
		students = append(students, student)
	}

	tmpl.Execute(w, students)
}

func getStudents() []Student {
	var students []Student
	for _, student := range studentData {
		students = append(students, student)
	}
	return students
}
