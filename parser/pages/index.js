import styles from '../styles/Home.module.css';
import { useState, useEffect } from 'react';
import * as sdpTransform from 'sdp-transform';

export default function Home() {
    let [message, setMessage] = useState("")

    useEffect(() => {
        let socket = new WebSocket("ws://localhost:8443/observer")
        let str = "";
        socket.addEventListener('message', function (event) {
            let m = JSON.parse(event.data);
            console.log("recv ", m);
            let type = m["type"];
            let data = m["data"];
            if (type == "sdp") {
                data["sdp"].replaceAll("\\r\\n", "\r\n");
                let sdp = sdpTransform.parse(data["sdp"]);
                console.log(sdp);
                str += '\n\n' + data["sdp"];
            }
            console.log(str)
            setMessage(str);
        });
    }, []);

    return (
        <div className={styles.container}>
            <textarea value={message} readOnly={true}>
            </textarea>
        </div >
    )
}
