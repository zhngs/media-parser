let localVideo;
let localStream;
let remoteVideo;
let peerConnection;
let uuid;
let serverConnection;

// 一般使用这个配置
let peerConnectionConfig = {
  // 'iceServers': [
  //   { 'urls': 'stun:stun.stunprotocol.org:3478' },
  //   { 'urls': 'stun:stun.l.google.com:19302' },
  // ]
};

// 用于测试turn服务器
// let peerConnectionConfig = {
//     'iceTransportPolicy':"relay",
//     'iceServers': [
//         {'urls': 'turn:www.xxx.com:8444?transport=udp', 'username': 'webrtc-demo', 'credential': '123456'},
//     ] 
// }

async function onSignalMessage(message) {
  if (!peerConnection) start(false);

  let signal = JSON.parse(message.data);
  console.log(signal.sdp.sdp);

  // Ignore messages from ourself
  if (signal.uuid == uuid) return;

  try {
    await peerConnection.setRemoteDescription(new RTCSessionDescription(signal.sdp));
    if (signal.sdp.type == 'offer') {
      let answer = await peerConnection.createAnswer()
      await peerConnection.setLocalDescription(answer)
    }
  } catch (err) {
    console.log(err)
  }
}

async function pageReady() {
  uuid = createUUID();

  localVideo = document.getElementById('localVideo');
  remoteVideo = document.getElementById('remoteVideo');

  serverConnection = new WebSocket('ws://' + window.location.hostname + ':8443/signal');
  serverConnection.onmessage = onSignalMessage;

  let constraints = {
    video: true,
    audio: true,
  };

  try {
    localStream = await navigator.mediaDevices.getUserMedia(constraints)
    localVideo.srcObject = localStream
  } catch (err) {
    console.log(err)
  }
}

async function start(isCaller) {
  peerConnection = new RTCPeerConnection(peerConnectionConfig);

  peerConnection.onicecandidate = event => {
    console.log(event.candidate)
    if (event.candidate === null) {
      console.log(peerConnection.localDescription.sdp)
      serverConnection.send(JSON.stringify({ 'sdp': peerConnection.localDescription, 'uuid': uuid }));
    }
  };

  peerConnection.ontrack = event => {
    console.log('got remote stream');
    remoteVideo.srcObject = event.streams[0];
  };

  peerConnection.addStream(localStream);

  if (isCaller) {
    try {
      let offer = await peerConnection.createOffer()
      await peerConnection.setLocalDescription(offer)
      // serverConnection.send(JSON.stringify({ 'sdp': peerConnection.localDescription, 'uuid': uuid }));
    } catch (err) {
      console.log(err)
    }
  }
}

// Taken from http://stackoverflow.com/a/105074/515584
// Strictly speaking, it's not a real UUID, but it gets the job done here
function createUUID() {
  function s4() {
    return Math.floor((1 + Math.random()) * 0x10000).toString(16).substring(1);
  }

  return s4() + s4() + '-' + s4() + '-' + s4() + '-' + s4() + '-' + s4() + s4() + s4();
}