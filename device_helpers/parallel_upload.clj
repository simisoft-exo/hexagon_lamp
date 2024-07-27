#!/usr/bin/env bb

(require '[cheshire.core :as json]
         '[clojure.string :as str])

(import '[java.io InputStreamReader BufferedReader])

(def device-mappings (json/parse-string (slurp "device_mappings.json")))

(defn run-command [& args]
  (try
    (let [process (.exec (Runtime/getRuntime) (into-array String args))
          exit-code (.waitFor process)
          output-reader (BufferedReader. (InputStreamReader. (.getInputStream process)))
          error-reader (BufferedReader. (InputStreamReader. (.getErrorStream process)))
          output (slurp output-reader)
          error (slurp error-reader)]
      {:exit exit-code :out output :err error})
    (catch Exception e
      {:exit 1 :out "" :err (.getMessage e)})))

(defn upload-to-device [device]
  (let [serial (get device "device_serial_no")
        device-id (get device "device_id")
        cmd ["/usr/bin/st-flash" "--serial" serial "write" "/tmp/arduino/sketches/9966A1D8593F74EE345AA9DA5FF68892/simplefoc_tuning.ino.bin" "0x08000000"]
        process (future
                  (let [result (apply run-command cmd)]
                    (if (zero? (:exit result))
                      (str "Device " device-id " (Serial: " serial "): Upload successful")
                      (str "Device " device-id " (Serial: " serial "): Upload failed - " (str/trim (:err result))))))]
    {:device-id device-id
     :serial serial
     :process process}))

(defn update-status-lines [statuses]
  (print (str "\033[2J\033[H"))  ; Clear screen and move cursor to top-left
  (doseq [status statuses]
    (let [status-str (if (realized? (:process status))
                       @(:process status)
                       (str "Device " (:device-id status) " (Serial: " (:serial status) "): Uploading..."))]
      (println status-str)))
  (flush))

(let [upload-processes (map upload-to-device device-mappings)]
  (loop [remaining-processes upload-processes]
    (when (seq remaining-processes)
      (update-status-lines remaining-processes)
      (Thread/sleep 1000)
      (recur (filter #(not (realized? (:process %))) remaining-processes))))
  
  (update-status-lines upload-processes))

(shutdown-agents)