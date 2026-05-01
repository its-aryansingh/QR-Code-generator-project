import { BarChart, Bar, XAxis, YAxis, Tooltip, ResponsiveContainer, Cell } from 'recharts';

interface ReferrerTableProps {
    referrerData?: { referrer: string; count: number }[];
}

export function ReferrerTable({ referrerData }: ReferrerTableProps) {
    const data = referrerData || [];
    const maxCount = Math.max(...data.map(d => d.count), 1);

    return (
        <div className="bg-[#12121a] border border-white/10 rounded-xl p-6 h-full flex flex-col">
            <h3 className="text-lg font-semibold text-white mb-4 flex items-center gap-2">
                <svg className="w-5 h-5 text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" /></svg>
                Top Referrers
            </h3>

            <div className="flex-1 overflow-y-auto space-y-4 custom-scrollbar pr-2">
                {data.length > 0 ? (
                    data.map((item, index) => (
                        <div key={index} className="group">
                            <div className="flex justify-between text-sm mb-1">
                                <span className="text-slate-300 font-medium truncate max-w-[70%]" title={item.referrer}>{item.referrer || "Direct / Unknown"}</span>
                                <span className="text-slate-400">{item.count} <span className="text-[10px]">scans</span></span>
                            </div>
                            <div className="w-full bg-white/5 rounded-full h-1.5 overflow-hidden">
                                <div
                                    className="bg-indigo-500 h-1.5 rounded-full transition-all duration-500 group-hover:bg-indigo-400"
                                    style={{ width: `${(item.count / maxCount) * 100}%` }}
                                ></div>
                            </div>
                        </div>
                    ))
                ) : (
                    <div className="h-full flex flex-col items-center justify-center text-slate-500 gap-2">
                        <div className="w-12 h-12 rounded-full bg-white/5 flex items-center justify-center text-2xl">🔗</div>
                        <p>No query data yet</p>
                    </div>
                )}
            </div>
        </div>
    );
}
