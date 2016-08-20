(ns echo.core
  (:gen-class)
  (:require [clojure.string :as cs]
            [clojure.java.io :as io])
  (:import org.apache.commons.codec.binary.Hex
           java.io.BufferedReader
           java.io.IOException
           java.io.InputStreamReader))


(defn transform-http-msg
  "Function that transforms/filters the incoming HTTP messages."
  [headers body]
  ;; do actual transformations here
  [headers body])


(defn decode-hex-string
  "Decode an Hex-encoded string."
  [s]
  (String. (Hex/decodeHex (.toCharArray s))))


(defn encode-hex-string
  "Encode a string to a hex-encoded string."
  [^String s]
  (String. (Hex/encodeHex (.getBytes s))))


(defn -main
  [& args]
  (let [br (BufferedReader. (InputStreamReader. System/in))]
    (try
      (loop [hex-line (.readLine br)]
        (let [decoded-req (decode-hex-string hex-line)

              ;; empty line separates headers from body
              http-request (partition-by empty? (cs/split-lines decoded-req))
              headers (first http-request)

              ;; HTTP messages can contain no body:
              body (when (= 3 (count http-request)) (last http-request))
              [new-headers new-body] (transform-http-msg headers body)]

          (println (encode-hex-string (str (cs/join "\n" headers)
                                           (when body
                                             (str "\n\n"
                                                  (cs/join "\n" body)))))))
        (when-let [line (.readLine br)]
          (recur line)))
      (catch IOException e nil))))


