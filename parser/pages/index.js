import styles from '../styles/Home.module.css';
import { useState, useEffect } from 'react';

export default function Home() {
    let [message, setMessage] = useState("")

    useEffect(() => {
        let socket = new WebSocket("ws://localhost:8443/observer")
        let str = "";
        socket.addEventListener('message', function (event) {
            message = JSON.parse(event.data);
            console.log("recv ", message);
            let type = message["type"];
            let data = message["data"];
            if (type == "sdp") {
                data["sdp"].replaceAll("\\r\\n", "\r\n");
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
