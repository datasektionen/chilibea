function Modal(id) {
  const elem = document.getElementById(id);
  if (elem && elem.open) {
    elem.close();
  } else {
    elem.showModal();
  }
  console.log("modal");
}

function toggleFullscreen() {
  const elem = document.documentElement;
  if (!document.fullscreenElement) {
    if (elem.requestFullscreen) {
      elem.requestFullscreen();
    } else if (elem.webkitRequestFullscreen) {
      /* Safari */
      elem.webkitRequestFullscreen();
    } else if (elem.msRequestFullscreen) {
      /* IE11 */
      elem.msRequestFullscreen();
    }
  } else {
    if (document.exitFullscreen) {
      document.exitFullscreen();
    } else if (document.webkitExitFullscreen) {
      /* Safari */
      document.webkitExitFullscreen();
    } else if (document.msExitFullscreen) {
      /* IE11 */
      document.msExitFullscreen();
    }
  }
}

function highlight(id) {
  const places = [
    "Me1",
    "Me2",
    "D1",
    "D2",
    "Ghost",
    "Cubb",
    "Mdor",
    "Sjuk",
    "Kit",
    "Meet",
    "Prop",
    "Soda",
  ];
  places.forEach((place) => {
    const elem = document.getElementById(place);
    if (elem && place === id) {
      elem.classList.add("highlight");
    } else if (elem) {
      elem.classList.remove("highlight");
    }
  });
}
