interface LocationTableProps {
    cityData?: { city: string; country_code: string; count: number }[];
    countryData?: { country_name: string; country_code: string; count: number }[];
}

export function LocationTable({ cityData, countryData }: LocationTableProps) {
    const maxCityCount = cityData && cityData.length > 0 ? Math.max(...cityData.map(c => c.count)) : 0;
    const maxCountryCount = countryData && countryData.length > 0 ? Math.max(...countryData.map(c => c.count)) : 0;

    return (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4 h-full">
            {/* Top Countries */}
            <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 overflow-hidden flex flex-col">
                <h3 className="text-md font-semibold text-white mb-4 flex items-center gap-2">
                    <span>🌍</span> Top Countries
                </h3>
                <div className="flex-1 overflow-y-auto pr-2 space-y-3 custom-scrollbar">
                    {countryData && countryData.length > 0 ? (
                        countryData.map((item, index) => (
                            <div key={index} className="space-y-1">
                                <div className="flex justify-between text-sm">
                                    <span className="text-slate-200 font-medium flex items-center gap-2">
                                        {/* Using flag emoji based on country code would be nice, for now just code */}
                                        <span className="text-xs bg-white/10 px-1.5 py-0.5 rounded text-slate-400 font-mono">
                                            {item.country_code || "N/A"}
                                        </span>
                                        {item.country_name || "Unknown"}
                                    </span>
                                    <span className="text-slate-400">{item.count}</span>
                                </div>
                                <div className="h-1.5 bg-white/5 rounded-full overflow-hidden">
                                    <div
                                        className="h-full bg-gradient-to-r from-violet-500 to-fuchsia-500 rounded-full"
                                        style={{ width: `${(item.count / maxCountryCount) * 100}%` }}
                                    ></div>
                                </div>
                            </div>
                        ))
                    ) : (
                        <div className="text-center text-slate-500 py-8 text-sm">No location data yet</div>
                    )}
                </div>
            </div>

            {/* Top Cities */}
            <div className="bg-[#12121a] border border-white/10 rounded-xl p-4 overflow-hidden flex flex-col">
                <h3 className="text-md font-semibold text-white mb-4 flex items-center gap-2">
                    <span>🏙️</span> Top Cities
                </h3>
                <div className="flex-1 overflow-y-auto pr-2 space-y-3 custom-scrollbar">
                    {cityData && cityData.length > 0 ? (
                        cityData.map((item, index) => (
                            <div key={index} className="space-y-1">
                                <div className="flex justify-between text-sm">
                                    <span className="text-slate-200 font-medium">
                                        {item.city || "Unknown City"}
                                    </span>
                                    <span className="text-slate-400">{item.count}</span>
                                </div>
                                <div className="h-1.5 bg-white/5 rounded-full overflow-hidden">
                                    <div
                                        className="h-full bg-gradient-to-r from-cyan-500 to-blue-500 rounded-full"
                                        style={{ width: `${(item.count / maxCityCount) * 100}%` }}
                                    ></div>
                                </div>
                            </div>
                        ))
                    ) : (
                        <div className="text-center text-slate-500 py-8 text-sm">No city data yet</div>
                    )}
                </div>
            </div>
        </div>
    );
}
