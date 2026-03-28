# webmwall
![](https://raw.githubusercontent.com/Koumbaya/webmwall/refs/heads/main/readme/example.gif)


A full screen webm/images wall display. When launched in a folder it opens up a web server that displays the content of the folder in an alternating grid layout that's auto scrolling.

### Features: 
- Items can be pinned so that they are static relative to the rest of the display 📌
- Items can be deleted while browsing 🚮
- The number of columns can be adjusted ↔️
- Speed and scrolling direction (alternate/up/down) are adjustable.
- Ability to quickly filter by filetype.
- Double clicking an item put it in fullscreen.
- Auto-off: closing the tab will let the server auto-exit after a short while.

![](https://raw.githubusercontent.com/Koumbaya/webmwall/refs/heads/main/readme/toolbar.png)

Supported filetypes: webm,mp4,gif,webp,png,bmp,jpg

Can handle tens of thousands of images/videos with a low footprint (takes around 350Mb of RAM in firefox for 24k+ images and fast scrolling, objects are garbage collected).


The front-end part was done by copilot then claude. Backend mostly copilot for the first draft then manually.