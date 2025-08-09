const { contextBridge, ipcRenderer } = require('electron');

console.log('[preload] Executed preload.js');

contextBridge.exposeInMainWorld('electronAPI', {
    onFullscreenChange: (callback) => {
        console.log('[preload] Setting up fullscreen listener');
        ipcRenderer.on('fullscreen-changed', (event, isFullscreen) => {
            console.log('[preload] fullscreen-changed received:', isFullscreen);
            callback(isFullscreen);
        });
    },
    isFullscreen: async () => {
        console.log('[preload] Invoking check-fullscreen');
        const result = await ipcRenderer.invoke('check-fullscreen');
        console.log('[preload] check-fullscreen returned:', result);
        return result;
    }
});
