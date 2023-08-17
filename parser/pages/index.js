import styles from '../styles/Home.module.css';
import {useState} from 'react';

export default function Home() {
  let [message, setMessage] = useState("")

  let socket = new WebSocket("ws://localhost:8443/observer")

  socket.addEventListener('message', function (event) {
      console.log('Message from server ', event.data);
      setMessage(event.data);
  });

  return (
    <div className={styles.container}>
      {message}
    </div>
  )
}
